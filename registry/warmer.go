// Runs a daemon to continuously warm the registry cache.
package registry

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
)

type Warmer struct {
	Logger        log.Logger
	ClientFactory ClientFactory
	Username      string
	Expiry        time.Duration
	Client        MemcacheClient
}

// Continuously wait for a new repository to warm
func (w *Warmer) Loop(stop chan struct{}, wg *sync.WaitGroup, warm chan Repository) {
	if w.Logger == nil || w.ClientFactory == nil || w.Expiry == 0 || w.Client == nil {
		panic("registry.Warmer fields are nil")
	}

	defer wg.Done()
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

	// Refresh tags first
	tags, err := client.Tags(repository.String())
	if err != nil {
		w.Logger.Log("err", err.Error())
		return
	}

	val, err := json.Marshal(tags)
	if err != nil {
		w.Logger.Log("err", errors.Wrap(err, "serializing tags to store in memcache"))
		return
	}

	key := tagKey(w.Username, repository.String())

	if err := w.Client.Set(&memcache.Item{
		Key:        key,
		Value:      val,
		Expiration: int32(w.Expiry.Seconds()),
	}); err != nil {
		w.Logger.Log("err", errors.Wrap(err, "storing tags in memcache"))
		return
	}

	// Now refresh the manifests for each tag
	for _, tag := range tags {
		history, err := client.Manifest(repository.String(), tag)

		val, err := json.Marshal(history)
		if err != nil {
			w.Logger.Log("err", errors.Wrap(err, "serializing tag to store in memcache"))
			return
		}

		key := manifestKey(w.Username, repository.String(), tag)
		if err := w.Client.Set(&memcache.Item{
			Key:        key,
			Value:      val,
			Expiration: int32(w.Expiry.Seconds()),
		}); err != nil {
			w.Logger.Log("err", errors.Wrap(err, "storing tags in memcache"))
			return
		}
	}
}
