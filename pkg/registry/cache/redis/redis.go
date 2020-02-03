package redis

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-redis/redis/v7"
	"github.com/pkg/errors"

	"github.com/fluxcd/flux/pkg/registry/cache"
)

type RedisClient struct {
	logger log.Logger

	client *redis.Client
	quit   chan struct{}
	wait   sync.WaitGroup
}

func (r *RedisClient) GetKey(k cache.Keyer) ([]byte, time.Time, error) {
	v := r.client.Get(k.Key())
	ci, err := v.Bytes()
	if err == redis.Nil {
		// cache miss, no need of logging
		return nil, time.Time{}, cache.ErrNotCached
	} else if err != nil {
		// error interacting with Redis
		_ = r.logger.Log("err", errors.Wrap(err, "fetching tag from redis"))
		return nil, time.Time{}, err
	}

	return cache.EndianGet(ci)
}

func (r *RedisClient) SetKey(k cache.Keyer, deadline time.Time, v []byte) (err error) {
	expiry := cache.GracePeriodDeadline(deadline)
	deadlineBytes := cache.EndianPut(deadline)

	if _, err = r.client.Set(k.Key(), cache.EndianCompose(deadlineBytes, v), expiry).Result(); err != nil {
		_ = r.logger.Log("err", errors.Wrap(err, "storing in redis"))
		return err
	}
	return
}

type RedisConfig struct {
	Service  string
	Port     int
	Timeout  time.Duration
	MaxConns int
	Logger   log.Logger
}

func NewRedisClient(config RedisConfig) *RedisClient {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", config.Service, config.Port),
		Password:     "", // no password set
		DB:           0,  // use default DB
		DialTimeout:  config.Timeout,
		ReadTimeout:  config.Timeout,
		WriteTimeout: config.Timeout,
		PoolSize:     config.MaxConns,
	})

	return &RedisClient{
		logger: config.Logger,
		client: client,
		quit:   nil,
		wait:   sync.WaitGroup{},
	}
}
