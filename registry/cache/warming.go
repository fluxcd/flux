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

const askForNewImagesInterval = time.Minute

// start off assuming an image will change about an hour from first
// seeing it
const initialRefresh = 1 * time.Hour

// never try to refresh a tag faster than this
const minRefresh = 5 * time.Minute

// never set a refresh deadline longer than this
const maxRefresh = 7 * 24 * time.Hour

// excluded images get an constant, fairly long refresh deadline; we
// don't expect them to become usable e.g., change architecture.
const excludedRefresh = 24 * time.Hour

// the whole set of image manifests for a repo gets a long refresh; in
// general we write it back every time we go 'round the loop, so this
// is mainly for the effect of making garbage collection less likely.
const repoRefresh = maxRefresh

func clipRefresh(r time.Duration) time.Duration {
	if r > maxRefresh {
		return maxRefresh
	}
	if r < minRefresh {
		return minRefresh
	}
	return r
}

// Warmer refreshes the information kept in the cache from remote
// registries.
type Warmer struct {
	clientFactory registry.ClientFactory
	cache         Client
	burst         int
	Trace         bool
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
	priorityWarm := func(name image.Name) {
		logger.Log("priority", name.String())
		if creds, ok := imageCreds[name]; ok {
			w.warm(ctx, time.Now(), logger, name, creds)
		} else {
			logger.Log("priority", name.String(), "err", "no creds available")
		}
	}

	// This loop acts keeps a kind of priority queue, whereby image
	// names coming in on the `Priority` channel are looked up first.
	// If there are none, images used in the cluster are refreshed;
	// but no more often than once every `askForNewImagesInterval`,
	// since there is no effective back-pressure on cache refreshes
	// and it would spin freely otherwise.
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
			w.warm(ctx, time.Now(), logger, im.Name, im.Credentials)
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

func (w *Warmer) warm(ctx context.Context, now time.Time, logger log.Logger, id image.Name, creds registry.Credentials) {
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
			err = w.cache.SetKey(repoKey, now.Add(repoRefresh), bytes)
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

	// Create a list of images that need updating
	type update struct {
		ref             image.Ref
		previousDigest  string
		previousRefresh time.Duration
	}
	var toUpdate []update

	// Counters for reporting what happened
	var missing, refresh int
	for _, tag := range tags {
		if tag == "" {
			errorLogger.Log("err", "empty tag in fetched tags", "tags", tags)
			repo.LastError = "empty tag in fetched tags"
			return // abort and let the error be written
		}

		// See if we have the manifest already cached
		newID := id.ToRef(tag)
		key := NewManifestKey(newID.CanonicalRef())
		bytes, deadline, err := w.cache.GetKey(key)
		// If err, then we don't have it yet. Update.
		switch {
		case err != nil: // by and large these are cache misses, but any error shall count as "not found"
			if err != ErrNotCached {
				errorLogger.Log("warning", "error from cache", "err", err, "ref", newID)
			}
			missing++
			toUpdate = append(toUpdate, update{ref: newID, previousRefresh: initialRefresh})
		case len(bytes) == 0:
			errorLogger.Log("warning", "empty result from cache", "ref", newID)
			missing++
			toUpdate = append(toUpdate, update{ref: newID, previousRefresh: initialRefresh})
		default:
			var entry registry.ImageEntry
			if err := json.Unmarshal(bytes, &entry); err == nil {
				if w.Trace {
					errorLogger.Log("trace", "found cached manifest", "ref", newID, "last_fetched", entry.LastFetched.Format(time.RFC3339), "deadline", deadline.Format(time.RFC3339))
				}

				if entry.ExcludedReason == "" {
					newImages[tag] = entry.Info
					if now.After(deadline) {
						previousRefresh := minRefresh
						lastFetched := entry.Info.LastFetched
						if !lastFetched.IsZero() {
							previousRefresh = deadline.Sub(lastFetched)
						}
						toUpdate = append(toUpdate, update{ref: newID, previousRefresh: previousRefresh, previousDigest: entry.Info.Digest})
						refresh++
					}
				} else {
					if w.Trace {
						logger.Log("trace", "excluded in cache", "ref", newID, "reason", entry.ExcludedReason)
					}
					if now.After(deadline) {
						toUpdate = append(toUpdate, update{ref: newID, previousRefresh: excludedRefresh})
						refresh++
					}
				}
			}
		}
	}

	var fetchMx sync.Mutex // also guards access to newImages
	var successCount int

	if len(toUpdate) > 0 {
		logger.Log("info", "refreshing image", "image", id, "tag_count", len(tags), "to_update", len(toUpdate), "of_which_refresh", refresh, "of_which_missing", missing)

		// The upper bound for concurrent fetches against a single host is
		// w.Burst, so limit the number of fetching goroutines to that.
		fetchers := make(chan struct{}, w.burst)
		awaitFetchers := &sync.WaitGroup{}
		awaitFetchers.Add(len(toUpdate))

		ctxc, cancel := context.WithCancel(ctx)
		var once sync.Once
		defer cancel()

	updates:
		for _, up := range toUpdate {
			select {
			case <-ctxc.Done():
				break updates
			case fetchers <- struct{}{}:
			}

			go func(update update) {
				defer func() { awaitFetchers.Done(); <-fetchers }()

				imageID := update.ref

				if w.Trace {
					errorLogger.Log("trace", "refreshing manifest", "ref", imageID, "previous_refresh", update.previousRefresh.String())
				}

				// Get the image from the remote
				entry, err := client.Manifest(ctxc, imageID.Tag)
				if err != nil {
					if err, ok := errors.Cause(err).(net.Error); ok && err.Timeout() {
						// This was due to a context timeout, don't bother logging
						return
					}

					// abort the image tags fetching if we've been rate limited
					if strings.Contains(err.Error(), "429") {
						once.Do(func() {
							errorLogger.Log("warn", "aborting image tag fetching due to rate limiting, will try again later")
						})
						cancel()
					} else {
						errorLogger.Log("err", err, "ref", imageID)
					}
					return
				}

				refresh := update.previousRefresh
				reason := ""
				switch {
				case entry.ExcludedReason != "":
					errorLogger.Log("excluded", entry.ExcludedReason, "ref", imageID)
					refresh = excludedRefresh
					reason = "image is excluded"
				case update.previousDigest == "":
					entry.Info.LastFetched = now
					refresh = update.previousRefresh
					reason = "no prior cache entry for image"
				case entry.Info.Digest == update.previousDigest:
					entry.Info.LastFetched = now
					refresh = clipRefresh(refresh * 2)
					reason = "image digest is same"
				default: // i.e., not excluded, but the digests differ -> the tag was moved
					entry.Info.LastFetched = now
					refresh = clipRefresh(refresh / 2)
					reason = "image digest is different"
				}

				if w.Trace {
					errorLogger.Log("trace", "caching manifest", "ref", imageID, "last_fetched", now.Format(time.RFC3339), "refresh", refresh.String(), "reason", reason)
				}

				key := NewManifestKey(imageID.CanonicalRef())
				// Write back to memcached
				val, err := json.Marshal(entry)
				if err != nil {
					errorLogger.Log("err", err, "ref", imageID)
					return
				}
				err = w.cache.SetKey(key, now.Add(refresh), val)
				if err != nil {
					errorLogger.Log("err", err, "ref", imageID)
					return
				}
				fetchMx.Lock()
				successCount++
				if entry.ExcludedReason == "" {
					newImages[imageID.Tag] = entry.Info
				}
				fetchMx.Unlock()
			}(up)
		}
		awaitFetchers.Wait()
		logger.Log("updated", id.String(), "successful", successCount, "attempted", len(toUpdate))
	}

	// We managed to fetch new metadata for everything we were missing
	// (if anything). Ratchet the result forward.
	if successCount == len(toUpdate) {
		repo = ImageRepository{
			LastUpdate: time.Now(),
			Images:     newImages,
		}
		// If we got through all that without bumping into `HTTP 429
		// Too Many Requests` (or other problems), we can potentially
		// creep the rate limit up
		w.clientFactory.Succeed(id.CanonicalName())
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
