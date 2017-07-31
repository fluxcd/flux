// Runs a daemon to continuously warm the registry cache.
package registry

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/registry/cache"
)

const refreshWhenExpiryWithin = time.Minute
const askForNewImagesInterval = time.Minute

type Warmer struct {
	Logger        log.Logger
	ClientFactory ClientFactory
	Creds         Credentials
	Expiry        time.Duration
	Writer        cache.Writer
	Reader        cache.Reader
	Burst         int
}

type ImageCreds struct {
	ID    flux.ImageID
	Creds Credentials
}

// Continuously get the images to populate the cache with, and
// populate the cache with them.
func (w *Warmer) Loop(stop <-chan struct{}, wg *sync.WaitGroup, imagesToFetchFunc func() []ImageCreds) {
	defer wg.Done()

	if w.Logger == nil || w.ClientFactory == nil || w.Expiry == 0 || w.Writer == nil || w.Reader == nil {
		panic("registry.Warmer fields are nil")
	}

	for _, r := range imagesToFetchFunc() {
		w.warm(r)
	}

	newImages := time.Tick(askForNewImagesInterval)
	for {
		select {
		case <-stop:
			w.Logger.Log("stopping", "true")
			return
		case <-newImages:
			for _, r := range imagesToFetchFunc() {
				w.warm(r)
			}
		}
	}
}

func (w *Warmer) warm(img ImageCreds) {
	id := img.ID
	client, err := w.ClientFactory.ClientFor(id.Host, img.Creds)
	if err != nil {
		w.Logger.Log("err", err.Error())
		return
	}
	defer client.Cancel()

	username := w.Creds.credsFor(id.Host).username

	// Refresh tags first
	// Only, for example, "library/alpine" because we have the host information in the client above.
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

	key, err := cache.NewTagKey(username, id)
	if err != nil {
		w.Logger.Log("err", errors.Wrap(err, "creating key for cache"))
		return
	}

	err = w.Writer.SetKey(key, val)
	if err != nil {
		w.Logger.Log("err", errors.Wrap(err, "storing tags in cache"))
		return
	}

	// Create a list of manifests that need updating
	var toUpdate []flux.ImageID
	var expired bool
	for _, tag := range tags {
		// See if we have the manifest already cached
		// We don't want to re-download a manifest again.
		i := id.WithNewTag(tag)
		key, err := cache.NewManifestKey(username, i)
		if err != nil {
			w.Logger.Log("err", errors.Wrap(err, "creating key for memcache"))
			continue
		}
		expiry, err := w.Reader.GetExpiration(key)
		// If err, then we don't have it yet. Update.
		if err == nil { // If no error, we've already got it
			// If we're outside of the expiry buffer, skip, no need to update.
			if !withinExpiryBuffer(expiry, refreshWhenExpiryWithin) {
				continue
			}
			// If we're within the expiry buffer, we need to update quick!
			expired = true
		}
		toUpdate = append(toUpdate, i)
	}

	if len(toUpdate) == 0 {
		return
	}
	w.Logger.Log("fetching", id.String(), "to-update", len(toUpdate))

	if expired {
		w.Logger.Log("expiring", id.HostNamespaceImage())
	}

	// The upper bound for concurrent fetches against a single host is
	// w.Burst, so limit the number of fetching goroutines to that.
	fetchers := make(chan struct{}, w.Burst)
	awaitFetchers := &sync.WaitGroup{}
	for _, imID := range toUpdate {
		awaitFetchers.Add(1)
		fetchers <- struct{}{}
		go func(imageID flux.ImageID) {
			defer func() { awaitFetchers.Done(); <-fetchers }()
			// Get the image from the remote
			img, err := client.Manifest(imageID)
			if err != nil {
				if !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) && !strings.Contains(err.Error(), "net/http: request canceled") {
					w.Logger.Log("err", errors.Wrap(err, "requesting manifests"))
				}
				return
			}

			key, err := cache.NewManifestKey(username, img.ID)
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
			err = w.Writer.SetKey(key, val)
			if err != nil {
				w.Logger.Log("err", errors.Wrap(err, "storing manifests in cache"))
				return
			}
		}(imID)
	}
	awaitFetchers.Wait()
	w.Logger.Log("updated", id.HostNamespaceImage())
}

func withinExpiryBuffer(expiry time.Time, buffer time.Duration) bool {
	// if the `time.Now() + buffer  > expiry`,
	// then we're within the expiry buffer
	if time.Now().Add(buffer).After(expiry) {
		return true
	}
	return false
}
