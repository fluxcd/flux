// Runs a daemon to continuously warm the registry cache.
package registry

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
	"github.com/weaveworks/flux/registry/cache"
)

const refreshWhenExpiryWithin = time.Minute
const askForNewImagesInterval = time.Minute

type Warmer struct {
	Logger        log.Logger
	ClientFactory *RemoteClientFactory
	Expiry        time.Duration
	Cache         cache.Client
	Burst         int
	Priority      chan image.Name
	Notify        func()
}

// This is what we get from the callback handed to us
type ImageCreds map[image.Name]Credentials

// .. and this is what we keep in the backlog
type backlogItem struct {
	image.Name
	Credentials
}

// Continuously get the images to populate the cache with, and
// populate the cache with them.
func (w *Warmer) Loop(stop <-chan struct{}, wg *sync.WaitGroup, imagesToFetchFunc func() ImageCreds) {
	defer wg.Done()

	if w.Logger == nil || w.ClientFactory == nil || w.Expiry == 0 || w.Cache == nil {
		panic("registry.Warmer fields are nil")
	}

	refresh := time.Tick(askForNewImagesInterval)
	imageCreds := imagesToFetchFunc()
	backlog := imageCredsToBacklog(imageCreds)

	// This loop acts keeps a kind of priority queue, whereby image
	// names coming in on the `Priority` channel are looked up first.
	// If there are none, images used in the cluster are refreshed;
	// but no more often than once every `askForNewImagesInterval`,
	// since there is no effective back-pressure on cache refreshes
	// and it would spin freely otherwise).
	for {
		select {
		case <-stop:
			w.Logger.Log("stopping", "true")
			return
		case name := <-w.Priority:
			w.Logger.Log("priority", name.String())
			// NB the implicit contract here is that the prioritised
			// image has to have been running the last time we
			// requested the credentials.
			if creds, ok := imageCreds[name]; ok {
				w.warm(name, creds)
			} else {
				w.Logger.Log("priority", name.String(), "err", "no creds available")
			}
			continue
		default:
		}

		if len(backlog) > 0 {
			im := backlog[0]
			backlog = backlog[1:]
			w.warm(im.Name, im.Credentials)
		} else {
			select {
			case <-refresh:
				imageCreds = imagesToFetchFunc()
				backlog = imageCredsToBacklog(imageCreds)
			default:
			}
		}
	}
}

func imageCredsToBacklog(imageCreds ImageCreds) []backlogItem {
	backlog := make([]backlogItem, len(imageCreds))
	var i int
	for name, cred := range imageCreds {
		backlog[i] = backlogItem{name, cred}
		i++
	}
	return backlog
}

func (w *Warmer) warm(id image.Name, creds Credentials) {
	client, err := w.ClientFactory.ClientFor(id.Registry(), creds)
	if err != nil {
		w.Logger.Log("err", err.Error())
		return
	}
	defer client.Cancel()

	key, err := cache.NewTagKey(id.CanonicalName())
	if err != nil {
		w.Logger.Log("err", errors.Wrap(err, "creating key for cache"))
		return
	}

	var cacheTags []string
	cacheTagsVal, err := w.Cache.GetKey(key)
	if err == nil {
		err = json.Unmarshal(cacheTagsVal, &cacheTags)
		if err != nil {
			w.Logger.Log("err", errors.Wrap(err, "deserializing cached tags"))
			return
		}
	} // else assume we have no cached tags

	tags, err := client.Tags(id)
	if err != nil {
		if !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) && !strings.Contains(err.Error(), "net/http: request canceled") {
			w.Logger.Log("err", errors.Wrap(err, "requesting tags"))
		}
		return
	}

	val, err := json.Marshal(tags)
	if err != nil {
		w.Logger.Log("err", errors.Wrap(err, "serializing tags to store in cache"))
		return
	}

	err = w.Cache.SetKey(key, val)
	if err != nil {
		w.Logger.Log("err", errors.Wrap(err, "storing tags in cache"))
		return
	}

	// Create a list of manifests that need updating
	var toUpdate []image.Ref
	var expired bool
	for _, tag := range tags {
		// See if we have the manifest already cached
		// We don't want to re-download a manifest again.
		newID := id.ToRef(tag)
		key, err := cache.NewManifestKey(newID.CanonicalRef())
		if err != nil {
			w.Logger.Log("err", errors.Wrap(err, "creating key for memcache"))
			continue
		}
		expiry, err := w.Cache.GetExpiration(key)
		// If err, then we don't have it yet. Update.
		if err == nil { // If no error, we've already got it
			// If we're outside of the expiry buffer, skip, no need to update.
			if !withinExpiryBuffer(expiry, refreshWhenExpiryWithin) {
				continue
			}
			// If we're within the expiry buffer, we need to update quick!
			expired = true
		}
		toUpdate = append(toUpdate, newID)
	}

	if len(toUpdate) == 0 {
		return
	}
	w.Logger.Log("fetching", id.String(), "to-update", len(toUpdate))

	if expired {
		w.Logger.Log("expiring", id.String())
	}

	// The upper bound for concurrent fetches against a single host is
	// w.Burst, so limit the number of fetching goroutines to that.
	fetchers := make(chan struct{}, w.Burst)
	awaitFetchers := &sync.WaitGroup{}
	for _, imID := range toUpdate {
		awaitFetchers.Add(1)
		fetchers <- struct{}{}
		go func(imageID image.Ref) {
			defer func() { awaitFetchers.Done(); <-fetchers }()
			// Get the image from the remote
			img, err := client.Manifest(imageID)
			if err != nil {
				if err, ok := errors.Cause(err).(net.Error); ok && err.Timeout() {
					// This was due to a context timeout, don't bother logging
					return
				}
				w.Logger.Log("err", errors.Wrap(err, "requesting manifests"))
				return
			}

			key, err := cache.NewManifestKey(img.ID.CanonicalRef())
			if err != nil {
				w.Logger.Log("err", errors.Wrap(err, "creating key for memcache"))
				return
			}
			// Write back to memcache
			val, err := json.Marshal(img)
			if err != nil {
				w.Logger.Log("err", errors.Wrap(err, "serializing tag to store in cache"))
				return
			}
			err = w.Cache.SetKey(key, val)
			if err != nil {
				w.Logger.Log("err", errors.Wrap(err, "storing manifests in cache"))
				return
			}
		}(imID)
	}
	awaitFetchers.Wait()
	w.Logger.Log("updated", id.String())

	if w.Notify != nil {
		// If there's more tags than there used to be, there must be
		// at least one new tag.
		if len(cacheTags) < len(tags) {
			w.Notify()
			return
		}
		// Otherwise, check whether there are any entries in the
		// fetched tags that aren't in the cached tags.
		tagSet := NewStringSet(tags)
		cacheTagSet := NewStringSet(cacheTags)
		if !tagSet.Subset(cacheTagSet) {
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

func withinExpiryBuffer(expiry time.Time, buffer time.Duration) bool {
	// if the `time.Now() + buffer  > expiry`,
	// then we're within the expiry buffer
	if time.Now().Add(buffer).After(expiry) {
		return true
	}
	return false
}
