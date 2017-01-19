// +build integration

package registry

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/go-kit/kit/log"
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
	fmt.Printf("Memcache IPs: %v\n", strings.Fields(*memcachedIPs))
	mc := memcache.New(strings.Fields(*memcachedIPs)...)
	if err := mc.FlushAll(); err != nil {
		t.Fatal(err)
	}
	return &stoppableMemcacheClient{mc}
}

// Cleanup cleans up after a test
func Cleanup(t *testing.T) {}

type MockBackend struct {
	tags     func(repository string) ([]string, error)
	manifest func(repository, reference string) ([]schema1.History, error)
}

func (m *MockBackend) Tags(repository string) ([]string, error) {
	return m.tags(repository)
}

func (m *MockBackend) Manifest(repository, reference string) ([]schema1.History, error) {
	return m.manifest(repository, reference)
}

func TestCache(t *testing.T) {
	mc := Setup(t)
	defer Cleanup(t)

	manifestCalled := 0

	mock := &MockBackend{
		manifest: func(repo, ref string) ([]schema1.History, error) {
			manifestCalled++
			return []schema1.History{{`{"test":"json"}`}}, nil
		},
	}
	c := NewCache(
		NoCredentials(),
		mc,
		20*time.Minute,
		log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout)),
	)(mock)

	// It should fetch stuff from the backend
	response, err := c.Manifest("weaveworks/foorepo", "tag1")
	if err != nil {
		t.Fatal(err)
	}
	if len(response) != 1 {
		t.Fatalf("Expected 1 history item, got %v", response)
	}
	expected := schema1.History{`{"test":"json"}`}
	if response[0] != expected {
		t.Fatalf("Expected  history item: %v, got %v", expected, response[0])
	}
	if manifestCalled != 1 {
		t.Errorf("Expected 1 call to the backend, got %d", manifestCalled)
	}

	// It should cache on the way through
	_, err = mc.Get(strings.Join([]string{
		"registryhistoryv1",
		"", // no username
		"weaveworks/foorepo",
		"tag1",
	}, "|"))
	if err != nil {
		// Will catch ErrCacheMiss
		t.Fatal(err)
	}

	// it should prefer cached data
	_, err = c.Manifest("weaveworks/foorepo", "tag1")
	if err != nil {
		t.Fatal(err)
	}
	// still 1 call
	if manifestCalled != 1 {
		t.Errorf("Expected 1 call to the backend, got %d", manifestCalled)
	}
}
