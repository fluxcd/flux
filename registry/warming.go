// Runs a daemon to continuously warm the registry cache.
package registry

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/registry/cache"
	"math/rand"
	"strings"
)

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
		if !strings.Contains(err.Error(), "status=401") {
			w.Logger.Log("err", err.Error())
		}
		return
	}

	val, err := json.Marshal(tags)
	if err != nil {
		w.Logger.Log("err", errors.Wrap(err, "serializing tags to store in memcache"))
		return
	}

	key, err := cache.NewTagKey(username, id)
	if err != nil {
		w.Logger.Log("err", errors.Wrap(err, "creating key for memcache"))
		return
	}

	err = w.Writer.SetKey(key, val)
	if err != nil {
		w.Logger.Log("err", errors.Wrap(err, "storing tags in memcache"))
		return
	}

	// Now refresh the manifests for each tag
	var updated bool
	for _, tag := range tags {
		// See if we have the manifest already cached
		// We don't want to re-download a manifest again.
		key, err := cache.NewManifestKey(username, id, tag)
		if err != nil {
			w.Logger.Log("err", errors.Wrap(err, "creating key for memcache"))
			return
		}
		_, err = w.Reader.GetKey(key)
		if err == nil { // If no error, we've already got it, skip
			continue
		}

		// Get the image from the remote
		img, err := client.Manifest(id, tag)
		if err != nil {
			w.Logger.Log("err", err.Error())
			continue
		}

		// Write back to memcache
		val, err := json.Marshal(img)
		if err != nil {
			w.Logger.Log("err", errors.Wrap(err, "serializing tag to store in memcache"))
			return
		}
		err = w.Writer.SetKey(key, val)
		if err != nil {
			w.Logger.Log("err", errors.Wrap(err, "storing tags in memcache"))
			return
		}
		updated = true // Report that we've updated something
	}
	if updated {
		w.Logger.Log("updated", id.HostNamespaceImage())
	}
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
			c := w.RunningContainers()
			if len(c) > 0 { // Only add random registry if there are running containers
				i := rand.Intn(len(c)) // Pick random registry
				w.queueLock.Lock()
				w.warmQueue <- c[i] // Add registry to queue
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
