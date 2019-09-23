// +build integration

package memcached

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"

	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/registry"
	"github.com/fluxcd/flux/pkg/registry/cache"
	"github.com/fluxcd/flux/pkg/registry/middleware"
)

// memcachedIPs flag from memcached_test.go

// This tests a real memcached cache and a request to a real Docker
// repository. It is intended to be an end-to-end integration test for
// the warmer since I had a few bugs introduced when refactoring. This
// should cover against these bugs.
func TestWarming_WarmerWriteCacheRead(t *testing.T) {
	mc := NewFixedServerMemcacheClient(MemcacheConfig{
		Timeout:        time.Second,
		UpdateInterval: 1 * time.Minute,
		Logger:         log.With(log.NewLogfmtLogger(os.Stderr), "component", "memcached"),
	}, strings.Fields(*memcachedIPs)...)

	// This repo has a stable number of images in the low tens (just
	// <20); more than `burst` below, but not so many that timing out
	// is likely.
	// TODO(hidde): I temporary switched this to one of our images on
	// Docker Hub due to Quay.io outage. It is however not guaranteed
	// the amount of tags for this image stays stable and in the low
	// tens.
	id, _ := image.ParseRef("docker.io/weaveworks/flagger-loadtester")

	logger := log.NewLogfmtLogger(os.Stderr)

	remote := &registry.RemoteClientFactory{
		Logger:   log.With(logger, "component", "client"),
		Limiters: &middleware.RateLimiters{RPS: 10, Burst: 5},
		Trace:    true,
	}

	r := &cache.Cache{Reader: mc}

	w, _ := cache.NewWarmer(remote, mc, 125)
	shutdown := make(chan struct{})
	shutdownWg := &sync.WaitGroup{}
	defer func() {
		close(shutdown)
		shutdownWg.Wait()
	}()

	shutdownWg.Add(1)
	go w.Loop(log.With(logger, "component", "warmer"), shutdown, shutdownWg, func() registry.ImageCreds {
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
			_, err := r.GetImageRepositoryMetadata(id.Name)
			if err == nil {
				break Loop
			}
		}
	}

	repoMetadata, err := r.GetImageRepositoryMetadata(id.Name)
	assert.NoError(t, err)
	assert.True(t, len(repoMetadata.Images) > 0, "Length of returned images should be > 0")
	assert.Equal(t, len(repoMetadata.Images), len(repoMetadata.Tags), "the length of tags and images should match")

	for _, tag := range repoMetadata.Tags {
		i, ok := repoMetadata.Images[tag]
		assert.True(t, ok, "tag doesn't have image information %s", tag)
		// None of the images should have an empty ID or a creation time of zero
		assert.True(t, i.ID.String() != "" && i.ID.Tag != "", "Image should not have empty name or tag. Got: %q", i.ID.String())
		assert.NotZero(t, i.CreatedAt, "Created time should not be zero for image %q", i.ID.String())
	}
}
