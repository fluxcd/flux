// +build integration

package cache

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"
)

var (
	redisIp   = flag.String("redis-service", "127.0.0.1", "host for redis to connect to")
	redisPort = flag.Int("redis-port", 6379, "port for redis to connect to")
)

type keyerMock string

func (k keyerMock) Key() string {
	return string(k)
}

func newRedisClient() *RedisClient {
	return NewRedisClient(RedisConfig{
		Service: *redisIp,
		Port:    *redisPort,
		Timeout: time.Second,
		Logger:  log.With(log.NewLogfmtLogger(os.Stderr)),
	})
}

func TestRedisClient_CacheMiss(t *testing.T) {
	c := newRedisClient()
	rand.Seed(time.Now().Unix())
	k := keyerMock(fmt.Sprintf("random-%d-key", rand.Int31()))
	_, _, err := c.GetKey(k)

	assert.Error(t, err, "expecting error, got nil")
	assert.Equal(t, ErrNotCached.Error(), err.Error(), "expected cache miss, gor generic error")
}

func TestRedisClient_ExpiryReadWrite(t *testing.T) {
	c := newRedisClient()
	key := keyerMock("test")
	val := []byte("test bytes")

	defer func() { _, _ = c.client.Del(key.Key()).Result() }()

	// Set some dummy data
	now := time.Now().Round(time.Second)
	err := c.SetKey(key, now, val)
	assert.Nil(t, err, "expecting nil, got error")

	cached, deadline, err := c.GetKey(key)
	assert.Nil(t, err)
	assert.Equal(t, now, deadline)
	assert.Equal(t, string(cached), string(val))
}
