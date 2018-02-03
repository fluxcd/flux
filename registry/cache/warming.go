package cache

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/registry"
)

const refreshWhenExpiryWithin = time.Minute
const askForNewImagesInterval = time.Minute

// Warmer refreshes the information kept in the cache from remote
// registries.
type Warmer struct {
	clientFactory registry.ClientFactory
	cache         Client
	burst         int
	Priority      chan image.Name
	Notify        func()
}

// NewWarmer creates cache warmer that (when Loop is invoked) will
// periodically refresh the values kept in the cache.
func NewWarmer(cf registry.ClientFactory, cacheClient Client, burst int) (*Warmer, error) {
	if cf == nil || cacheClient == nil || burst <= 0 {
		return nil, errors.New("arguments must be non-nil (or > 0 in the case of burst)")
	}
	return &Warmer{
		clientFactory: cf,
		cache:         cacheClient,
		burst:         burst,
	}, nil
}

// .. and this is what we keep in the backlog
type backlogItem struct {
	image.Name
	registry.Credentials
}

// Loop continuously gets the images to populate the cache with,
// and populate the cache with them.
func (w *Warmer) Loop(logger log.Logger, stop <-chan struct{}, wg *sync.WaitGroup, imagesToFetchFunc func() registry.ImageCreds) {
	defer wg.Done()

	refresh := time.Tick(askForNewImagesInterval)
	imageCreds := imagesToFetchFunc()
	backlog := imageCredsToBacklog(imageCreds)

	// We have some fine control over how long to spend on each fetch
	// operation, since they are given a `context`. For now though,
	// just rattle through them one by one, however long they take.
	ctx := context.Background()

	// NB the implicit contract here is that the prioritised
	// image has to have been running the last time we
	// requested the credentials.
	priorityWarm := func (name image.Name) {
		logger.Log("priority", name.String())
		if creds, ok := imageCreds[name]; ok {
			w.warm(ctx, logger, name, creds)
		} else {
			logger.Log("priority", name.String(), "err", "no creds available")
		}
	}

	// This loop acts keeps a kind of priority queue, whereby image
	// names coming in on the `Priority` channel are looked up first.
	// If there are none, images used in the cluster are refreshed;
	// but no more often than once every `askForNewImagesInterval`,
	// since there is no effective back-pressure on cache refreshes
	// and it would spin freely otherwise).
	for {
		select {
		case <-stop:
			logger.Log("stopping", "true")
			return
		case name := <-w.Priority:
			priorityWarm(name)
			continue
		default:
		}

		if len(backlog) > 0 {
			im := backlog[0]
			backlog = backlog[1:]
			w.warm(ctx, logger, im.Name, im.Credentials)
		} else {
			select {
			case <-stop:
				logger.Log("stopping", "true")
				return 
			case <-refresh:
				imageCreds = imagesToFetchFunc()
				backlog = imageCredsToBacklog(imageCreds)
			case name := <-w.Priority:
				priorityWarm(name)
			}
		}
	}
}


func imageCredsToBacklog(imageCreds registry.ImageCreds) []backlogItem {
	backlog := make([]backlogItem, len(imageCreds))
	var i int
	for name, cred := range imageCreds {
		backlog[i] = backlogItem{name, cred}
		i++
	}
	return backlog
}

func (w *Warmer) warm(ctx context.Context, logger log.Logger, id image.Name, creds registry.Credentials) {
	errorLogger := log.With(logger, "canonical_name", id.CanonicalName(), "auth", creds)
	client, err := w.clientFactory.ClientFor(id.CanonicalName(), creds)
	if err != nil {
		errorLogger.Log("err", err.Error())
		return
	}

	// This is what we're going to write back to the cache
	var repo ImageRepository
	repoKey := NewRepositoryKey(id.CanonicalName())
	bytes, _, err := w.cache.GetKey(repoKey)
	if err == nil {
		err = json.Unmarshal(bytes, &repo)
	} else if err == ErrNotCached {
		err = nil
	}

	if err != nil {
		errorLogger.Log("err", errors.Wrap(err, "fetching previous result from cache"))
		return
	}
	// Save for comparison later
	oldImages := repo.Images

	// Now we have the previous result; everything after will be
	// attempting to refresh that value. Whatever happens, at the end
	// we'll write something back.
	defer func() {
		bytes, err := json.Marshal(repo)
		if err == nil {
			err = w.cache.SetKey(repoKey, bytes)
		}
		if err != nil {
			errorLogger.Log("err", errors.Wrap(err, "writing result to cache"))
		}
	}()

	tags, err := client.Tags(ctx)
	if err != nil {
		if !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) && !strings.Contains(err.Error(), "net/http: request canceled") {
			errorLogger.Log("err", errors.Wrap(err, "requesting tags"))
			repo.LastError = err.Error()
		}
		return
	}

	newImages := map[string]image.Info{}

	// Create a list of manifests that need updating
	var toUpdate []image.Ref
	var missing, expired int
	for _, tag := range tags {
		// See if we have the manifest already cached
		newID := id.ToRef(tag)
		key := NewManifestKey(newID.CanonicalRef())
		bytes, expiry, err := w.cache.GetKey(key)
		// If err, then we don't have it yet. Update.
		switch {
		case err != nil:
			missing++
		case time.Until(expiry) < refreshWhenExpiryWithin:
			expired++
		default:
			var image image.Info
			if err := json.Unmarshal(bytes, &image); err == nil {
				newImages[tag] = image
				continue
			}
			missing++
		}
		toUpdate = append(toUpdate, newID)
	}

	var successCount int

	if len(toUpdate) > 0 {
		logger.Log("fetching", id.String(), "total", len(toUpdate), "expired", expired, "missing", missing)
		var successMx sync.Mutex

		// The upper bound for concurrent fetches against a single host is
		// w.Burst, so limit the number of fetching goroutines to that.
		fetchers := make(chan struct{}, w.burst)
		awaitFetchers := &sync.WaitGroup{}
	updates:
		for _, imID := range toUpdate {
			select {
			case <-ctx.Done():
				break updates
			case fetchers <- struct{}{}:
			}

			awaitFetchers.Add(1)
			go func(imageID image.Ref) {
				defer func() { awaitFetchers.Done(); <-fetchers }()
				// Get the image from the remote
				img, err := client.Manifest(ctx, imageID.Tag)
				if err != nil {
					if err, ok := errors.Cause(err).(net.Error); ok && err.Timeout() {
						// This was due to a context timeout, don't bother logging
						return
					}
					errorLogger.Log("err", errors.Wrap(err, "requesting manifests"))
					return
				}

				key := NewManifestKey(img.ID.CanonicalRef())
				// Write back to memcached
				val, err := json.Marshal(img)
				if err != nil {
					errorLogger.Log("err", errors.Wrap(err, "serializing tag to store in cache"))
					return
				}
				err = w.cache.SetKey(key, val)
				if err != nil {
					errorLogger.Log("err", errors.Wrap(err, "storing manifests in cache"))
					return
				}
				successMx.Lock()
				successCount++
				newImages[imageID.Tag] = img
				successMx.Unlock()
			}(imID)
		}
		awaitFetchers.Wait()
		logger.Log("updated", id.String(), "count", successCount)
	}

	// We managed to fetch new metadata for everything we were missing
	// (if anything). Ratchet the result forward.
	if successCount == len(toUpdate) {
		repo = ImageRepository{
			LastUpdate: time.Now(),
			Images:     newImages,
		}
	}

	if w.Notify != nil {
		cacheTags := StringSet{}
		for t := range oldImages {
			cacheTags[t] = struct{}{}
		}

		// If there's more tags than there used to be, there must be
		// at least one new tag.
		if len(cacheTags) < len(tags) {
			w.Notify()
			return
		}
		// Otherwise, check whether there are any entries in the
		// fetched tags that aren't in the cached tags.
		tagSet := NewStringSet(tags)
		if !tagSet.Subset(cacheTags) {
			w.Notify()
		}
	}
}

// StringSet is a set of strings.
type StringSet map[string]struct{}

// NewStringSet returns a StringSet containing exactly the strings
// given as arguments.
func NewStringSet(ss []string) StringSet {
	res := StringSet{}
	for _, s := range ss {
		res[s] = struct{}{}
	}
	return res
}

// Subset returns true if `s` is a subset of `t` (including the case
// of having the same members).
func (s StringSet) Subset(t StringSet) bool {
	for k := range s {
		if _, ok := t[k]; !ok {
			return false
		}
	}
	return true
}
