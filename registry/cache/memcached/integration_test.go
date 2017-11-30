// +build integration

package memcached

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/registry"
	"github.com/weaveworks/flux/registry/cache"
	"github.com/weaveworks/flux/registry/middleware"
)

// memcachedIPs flag from memcached_test.go

// This tests a real memcached cache and a request to a real docker
// repository. It is intended to be an end-to-end integration test for
// the warmer since I had a few bugs introduced when refactoring. This
// should cover against these bugs.
func TestWarming_WarmerWriteCacheRead(t *testing.T) {
	mc := NewFixedServerMemcacheClient(MemcacheConfig{
		Timeout:        time.Second,
		UpdateInterval: 1 * time.Minute,
		Logger:         log.With(log.NewLogfmtLogger(os.Stderr), "component", "memcached"),
	}, strings.Fields(*memcachedIPs)...)

	id, _ := image.ParseRef("alpine")

	logger := log.NewLogfmtLogger(os.Stderr)

	remote := &registry.RemoteClientFactory{
		Logger:   log.With(logger, "component", "client"),
		Limiters: &middleware.RateLimiters{RPS: 200, Burst: 10},
		Trace:    true,
	}

	r := &cache.Cache{mc}

	w := &cache.Warmer{
		Logger:        log.With(logger, "component", "warmer"),
		ClientFactory: remote,
		Cache:         mc,
		Burst:         125,
	}

	shutdown := make(chan struct{})
	shutdownWg := &sync.WaitGroup{}
	defer func() {
		close(shutdown)
		shutdownWg.Wait()
	}()

	shutdownWg.Add(1)
	go w.Loop(shutdown, shutdownWg, func() registry.ImageCreds {
		return registry.ImageCreds{
			id.Name: registry.NoCredentials(),
		}
	})

	timeout := time.NewTicker(10 * time.Second)    // Shouldn't take longer than 10s
	tick := time.NewTicker(100 * time.Millisecond) // Check every 100ms

Loop:
	for {
		select {
		case <-timeout.C:
			t.Fatal("Cache timeout")
		case <-tick.C:
			_, err := r.GetRepository(id.Name)
			if err == nil {
				break Loop
			}
		}
	}

	img, err := r.GetRepository(id.Name)
	if err != nil {
		t.Fatal(err)
	}
	if len(img) == 0 {
		t.Fatal("Length of returned images should be > 0")
	}
	// None of the images should have an empty ID or a creation time of zero
	for _, i := range img {
		if i.ID.String() == "" || i.ID.Tag == "" {
			t.Fatalf("Image should not have empty name or tag. Got: %q", i.ID.String())
		}
		if i.CreatedAt.IsZero() {
			t.Fatalf("Created time should not be zero for image %q", i.ID.String())
		}
	}
}
