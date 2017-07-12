// +build integration

package cache

import (
	"flag"
	"github.com/go-kit/kit/log"
	"os"
	"strings"
	"testing"
	"time"
)

var (
	memcachedIPs = flag.String("memcached-ips", "127.0.0.1:11211", "space-separated host:port values for memcached to connect to")
)

var val = []byte("test bytes")

var key = testKey("test")

type testKey string

func (t testKey) String() string {
	return string(t)
}

func TestMemcache_ExpiryReadWrite(t *testing.T) {
	// Memcache client
	mc := NewFixedServerMemcacheClient(MemcacheConfig{
		Timeout:        time.Second,
		UpdateInterval: 1 * time.Minute,
		Logger:         log.NewContext(log.NewLogfmtLogger(os.Stderr)).With("component", "memcached"),
	}, strings.Fields(*memcachedIPs)...)

	// Set some dummy data
	err := mc.SetKey(key, val)
	if err != nil {
		t.Fatal(err)
	}

	// Get the expiry
	expiry, err := mc.GetExpiration(key)
	if err != nil {
		t.Fatal(err)
	}
	if expiry.IsZero() {
		t.Fatal("Time should not be zero")
	}
	if expiry.Before(time.Now()) {
		t.Fatal("Expiry should be in the future")
	}
}

func TestMemcache_ReadWrite(t *testing.T) {
	// Memcache client
	mc := NewFixedServerMemcacheClient(MemcacheConfig{
		Timeout:        time.Second,
		UpdateInterval: 1 * time.Minute,
		Logger:         log.NewContext(log.NewLogfmtLogger(os.Stderr)).With("component", "memcached"),
	}, strings.Fields(*memcachedIPs)...)

	// Set some dummy data
	err := mc.SetKey(key, val)
	if err != nil {
		t.Fatal(err)
	}

	// Get the data
	cached, err := mc.GetKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if string(cached) != string(val) {
		t.Fatalf("Should have returned %q, but got %q", string(val), string(cached))
	}
}
