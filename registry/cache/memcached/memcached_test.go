// +build integration

package memcached

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

func (t testKey) Key() string {
	return string(t)
}

func TestMemcache_ExpiryReadWrite(t *testing.T) {
	// Memcache client
	mc := NewFixedServerMemcacheClient(MemcacheConfig{
		Timeout:        time.Second,
		UpdateInterval: 1 * time.Minute,
		Logger:         log.With(log.NewLogfmtLogger(os.Stderr), "component", "memcached"),
	}, strings.Fields(*memcachedIPs)...)

	// Set some dummy data
	now := time.Now().Round(time.Second)
	err := mc.SetKey(key, now, val)
	if err != nil {
		t.Fatal(err)
	}

	cached, deadline, err := mc.GetKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if !deadline.Equal(now) {
		t.Fatalf("Deadline should be %s, but is %s", now.String(), deadline.String())
	}

	if string(cached) != string(val) {
		t.Fatalf("Should have returned %q, but got %q", string(val), string(cached))
	}
}
