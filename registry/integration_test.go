// +build integration

package registry

import (
	"flag"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/registry/cache"
	"github.com/weaveworks/flux/registry/middleware"
)

var (
	memcachedIPs = flag.String("memcached-ips", "127.0.0.1:11211", "space-separated host:port values for memcached to connect to")
)

// This tests a real memcache cache and a request to a real docker
// repository.  It is intended to be an end-to-end integration test
// for the warmer since I had a few bugs introduced when
// refactoring. This should cover against these bugs.
func TestWarming_WarmerWriteCacheRead(t *testing.T) {
	mc := cache.NewFixedServerMemcacheClient(cache.MemcacheConfig{
		Timeout:        time.Second,
		UpdateInterval: 1 * time.Minute,
		Logger:         log.With(log.NewLogfmtLogger(os.Stderr), "component", "memcached"),
	}, strings.Fields(*memcachedIPs)...)

	id, _ := image.ParseRef("alpine")

	logger := log.NewLogfmtLogger(os.Stderr)

	remote := NewRemoteClientFactory(
		log.With(logger, "component", "client"),
		middleware.RateLimiterConfig{200, 10},
	)

	cache := NewCacheClientFactory(
		log.With(logger, "component", "cache"),
		mc,
		time.Hour,
	)

	r := NewRegistry(
		cache,
		log.With(logger, "component", "registry"),
		512,
	)

	w := Warmer{
		Logger:        log.With(logger, "component", "warmer"),
		ClientFactory: remote,
		Creds:         NoCredentials(),
		Expiry:        time.Hour,
		Reader:        mc,
		Writer:        mc,
		Burst:         125,
	}

	shutdown := make(chan struct{})
	shutdownWg := &sync.WaitGroup{}
	defer func() {
		close(shutdown)
		shutdownWg.Wait()
	}()

	shutdownWg.Add(1)
	go w.Loop(shutdown, shutdownWg, func() ImageCreds {
		return ImageCreds{
			id.Name: NoCredentials(),
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
