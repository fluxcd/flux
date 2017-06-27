// +build integration

package registry

import (
	"encoding/json"
	"flag"
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/go-kit/kit/log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	memcachedIPs = flag.String("memcached-ips", "127.0.0.1:11211", "space-separated host:port values for memcached to connect to")
)

type stoppableMemcacheClient struct {
	*memcache.Client
}

func (s *stoppableMemcacheClient) Stop() {}

// Setup sets up stuff for testing
func Setup(t *testing.T) MemcacheClient {
	mc := memcache.New(strings.Fields(*memcachedIPs)...)
	if err := mc.FlushAll(); err != nil {
		t.Fatal(err)
	}
	return &stoppableMemcacheClient{mc}
}

func TestWarmer_CacheNewRepo(t *testing.T) {
	mc := Setup(t)
	defer mc.Stop()

	dc := NewMockDockerClient(
		func(repository, reference string) ([]schema1.History, error) {
			return []schema1.History{{`{"test":"json"}`}}, nil
		},
		func(repository string) ([]string, error) {
			return []string{"tag1"}, nil
		},
	)

	w := Warmer{
		Logger:        log.NewLogfmtLogger(os.Stderr),
		ClientFactory: &mockRemoteFactory{c: dc},
		Username:      "user",
		Expiry:        time.Hour,
		Client:        mc,
	}

	shutdown := make(chan struct{})
	repo := make(chan Repository)
	shutdownWg := &sync.WaitGroup{}
	shutdownWg.Add(1)
	go w.Loop(shutdown, repo, shutdownWg)

	r, _ := ParseRepository("test/repo")
	repo <- r

	shutdown <- struct{}{}
	shutdownWg.Wait()

	// Test that tags were written
	key := tagKey("user", r.String())
	item, err := mc.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	var tags []string
	err = json.Unmarshal(item.Value, &tags)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 {
		t.Fatalf("Expected 1 history item, got %v", tags)
	}
	expectedTag := "tag1"
	if tags[0] != expectedTag {
		t.Fatalf("Expected  history item: %v, got %v", expectedTag, tags[0])
	}

	// Test that manifest was written
	key = manifestKey("user", r.String(), "tag1")
	item, err = mc.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	var manifests []schema1.History
	err = json.Unmarshal(item.Value, &manifests)
	if err != nil {
		t.Fatal(err)
	}

	if len(manifests) != 1 {
		t.Fatalf("Expected 1 history item, got %v", manifests)
	}
	expectedManifest := schema1.History{`{"test":"json"}`}
	if manifests[0] != expectedManifest {
		t.Fatalf("Expected  history item: %v, got %v", expectedManifest, manifests[0])
	}
}
