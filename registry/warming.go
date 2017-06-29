// Runs a daemon to continuously warm the registry cache.
package registry

import (
	"encoding/json"
	"sync"
	"time"

	officialMemcache "github.com/bradfitz/gomemcache/memcache"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"github.com/weaveworks/flux/registry/memcache"
	"math/rand"
	"strings"
)

type Warmer struct {
	Logger        log.Logger
	ClientFactory ClientFactory
	Creds         Credentials
	Expiry        time.Duration
	Client        memcache.MemcacheClient
}

// Continuously wait for a new repository to warm
func (w *Warmer) Loop(stop <-chan struct{}, wg *sync.WaitGroup, warm <-chan Repository) {
	defer wg.Done()

	if w.Logger == nil || w.ClientFactory == nil || w.Expiry == 0 || w.Client == nil {
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

func (w *Warmer) warm(repository Repository) {
	client, cancel, err := w.ClientFactory.ClientFor(repository.Host())
	if err != nil {
		w.Logger.Log("err", err.Error())
		return
	}
	defer cancel()

	username := w.Creds.credsFor(repository.Host()).username

	// Refresh tags first
	// Only, for example, "library/alpine" because we have the host information in the client above.
	tags, err := client.Tags(repository.NamespaceImage())
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

	// Use the full path to image for the memcache key because there
	// might be duplicates from other registries
	key := tagKey(username, repository.String())

	if err := w.Client.Set(&officialMemcache.Item{
		Key:        key,
		Value:      val,
		Expiration: int32(w.Expiry.Seconds()),
	}); err != nil {
		w.Logger.Log("err", errors.Wrap(err, "storing tags in memcache"))
		return
	}

	// Now refresh the manifests for each tag
	for _, tag := range tags {
		// Only, for example, "library/alpine" because we have the host information in the client above.
		history, err := client.Manifest(repository.NamespaceImage(), tag)

		val, err := json.Marshal(history)
		if err != nil {
			w.Logger.Log("err", errors.Wrap(err, "serializing tag to store in memcache"))
			return
		}

		// Full path to image again.
		key := manifestKey(username, repository.String(), tag)
		if err := w.Client.Set(&officialMemcache.Item{
			Key:        key,
			Value:      val,
			Expiration: int32(w.Expiry.Seconds()),
		}); err != nil {
			w.Logger.Log("err", errors.Wrap(err, "storing tags in memcache"))
			return
		}
	}
}

// Queue provides an updating repository queue for the warmer.
// If no items are added to the queue this will randomly add a new
// registry to warm
type Queue struct {
	RunningContainers    func() []Repository
	Logger               log.Logger
	RegistryPollInterval time.Duration
	warmQueue            chan Repository
	queueLock            sync.Mutex
}

func NewQueue(runningContainersFunc func() []Repository, l log.Logger, emptyQueueTick time.Duration) Queue {
	return Queue{
		RunningContainers:    runningContainersFunc,
		Logger:               l,
		RegistryPollInterval: emptyQueueTick,
		warmQueue:            make(chan Repository, 100), // Don't close this. It will be GC'ed when this instance is destroyed.
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

func (w *Queue) Queue() chan Repository {
	w.queueLock.Lock()
	defer w.queueLock.Unlock()
	return w.warmQueue
}
