// Runs a daemon to continuously warm the registry cache.
package registry

import (
	"encoding/json"
	"sync"
	"time"

	"context"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/registry/cache"
	"strings"
)

const refreshWhenExpiryWithin = time.Minute

type Warmer struct {
	Logger        log.Logger
	ClientFactory ClientFactory
	Creds         Credentials
	Expiry        time.Duration
	Writer        cache.Writer
	Reader        cache.Reader
}

// Continuously wait for a new repository to warm
func (w *Warmer) Loop(stop <-chan struct{}, wg *sync.WaitGroup, warm <-chan flux.ImageID) {
	defer wg.Done()

	if w.Logger == nil || w.ClientFactory == nil || w.Expiry == 0 || w.Writer == nil || w.Reader == nil {
		panic("registry.Warmer fields are nil")
	}

	for {
		select {
		case <-stop:
			w.Logger.Log("stopping", "true")
			return
		case r := <-warm:
			w.warm(r)
		}
	}
}

func (w *Warmer) warm(id flux.ImageID) {
	client, err := w.ClientFactory.ClientFor(id.Host)
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

	if expired {
		w.Logger.Log("expiring", id.HostNamespaceImage())
	}

	// Now refresh the manifests for each tag (in lots of goroutines for improved performance)
	toFetch := make(chan flux.ImageID, len(toUpdate))
	fetched := make(chan flux.Image, len(toUpdate))
	for i := 0; i < MaxConcurrency; i++ {
		go func() {
			for i := range toFetch {
				// Get the image from the remote
				img, err := client.Manifest(i)
				if err != nil {
					if !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) && !strings.Contains(err.Error(), "net/http: request canceled") {
						w.Logger.Log("err", errors.Wrap(err, "requesting manifests"))
					}
				}
				fetched <- img // Always return an image, otherwise the for loop below will never finish.
			}
		}()
	}
	for _, img := range toUpdate {
		toFetch <- img
	}
	close(toFetch)

	// Write received manifests back to memcache
	for i := 0; i < cap(fetched); i++ {
		img := <-fetched
		if img.ID.String() == "" {
			continue
		}
		key, err := cache.NewManifestKey(username, img.ID)
		if err != nil {
			w.Logger.Log("err", errors.Wrap(err, "creating key for memcache"))
			continue
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
	}
	close(fetched)
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

// Queue provides an updating repository queue for the warmer.
// If no items are added to the queue this will randomly add a new
// registry to warm
type Queue struct {
	RunningContainers    func() []flux.ImageID
	Logger               log.Logger
	RegistryPollInterval time.Duration
	warmQueue            chan flux.ImageID
	queueLock            sync.Mutex
}

func NewQueue(runningContainersFunc func() []flux.ImageID, l log.Logger, emptyQueueTick time.Duration) Queue {
	return Queue{
		RunningContainers:    runningContainersFunc,
		Logger:               l,
		RegistryPollInterval: emptyQueueTick,
		warmQueue:            make(chan flux.ImageID, 100), // Don't close this. It will be GC'ed when this instance is destroyed.
	}
}

// Queue loop to maintain the queue and periodically add a random
// repository that is running in the cluster.
func (w *Queue) Loop(stop chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()

	if w.RunningContainers == nil || w.Logger == nil || w.RegistryPollInterval == 0 {
		panic("registry.Queue fields are nil")
	}

	pollImages := time.Tick(w.RegistryPollInterval)

	for {
		select {
		case <-stop:
			w.Logger.Log("stopping", "true")
			return
		case <-pollImages:
			if len(w.warmQueue) == 0 { // Only add to queue if queue is empty
				containers := w.RunningContainers() // Just add containers in order for now
				w.queueLock.Lock()
				for _, c := range containers {
					// if we can't write it immediately, drop it and move on
					select {
					case w.warmQueue <- c:
					default:
					}
				}
				w.queueLock.Unlock()
			}
		}
	}
}

func (w *Queue) Queue() chan flux.ImageID {
	w.queueLock.Lock()
	defer w.queueLock.Unlock()
	return w.warmQueue
}
