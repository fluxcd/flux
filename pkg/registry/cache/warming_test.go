package cache

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/docker/distribution/registry/api/errcode"
	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"

	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/registry"
	"github.com/fluxcd/flux/pkg/registry/mock"
)

type entry struct {
	b []byte
	d time.Time
}

type mem struct {
	kv map[string]entry
	mx sync.Mutex
}

var (
	ref  image.Ref
	repo image.Name
)

func init() {
	ref, _ = image.ParseRef("example.com/path/image:tag")
	repo = ref.Name
}

func (c *mem) SetKey(k Keyer, deadline time.Time, v []byte) error {
	println("set key", k.Key(), deadline.Format(time.RFC3339))
	c.mx.Lock()
	defer c.mx.Unlock()
	if c.kv == nil {
		c.kv = make(map[string]entry)
	}
	c.kv[k.Key()] = entry{v, deadline}
	return nil
}

func (c *mem) GetKey(k Keyer) ([]byte, time.Time, error) {
	c.mx.Lock()
	defer c.mx.Unlock()
	if c.kv == nil {
		c.kv = make(map[string]entry)
	}

	if e, ok := c.kv[k.Key()]; ok {
		println("get key", k.Key(), e.d.Format(time.RFC3339))
		return e.b, e.d, nil
	}
	println("get key", k.Key(), "nil")
	return nil, time.Time{}, ErrNotCached
}

// WarmTest effectively checks that the cache.Warmer and
// cache.Registry work together as intended: that is, if you ask the
// warmer to fetch information, the cached gets populated, and the
// Registry implementation will see it.
func TestWarmThenQuery(t *testing.T) {
	digest := "abc"
	warmer, cache := setup(t, &digest)
	logger := log.NewNopLogger()

	now := time.Now()
	warmer.warm(context.TODO(), now, logger, repo, registry.NoCredentials())

	registry := &Cache{Reader: cache}
	repoInfo, err := registry.GetImageRepositoryMetadata(ref.Name)
	assert.NoError(t, err)

	// Otherwise, we should get what we put in ...
	assert.Len(t, repoInfo.Tags, 1)
	assert.Equal(t, ref.String(), repoInfo.Images[repoInfo.Tags[0]].ID.String())
}

func TestWarmManifestUnknown(t *testing.T) {
	tagWithMissingMetadata := "4.0.8-r3"
	client := &mock.Client{
		TagsFn: func() ([]string, error) {
			println("asked for tags")
			return []string{tagWithMissingMetadata}, nil
		},
		ManifestFn: func(tag string) (registry.ImageEntry, error) {
			println("asked for manifest", tag)
			err := errcode.Errors{
				errcode.Error{Code: 1012,
					Message: "manifest unknown",
					Detail: map[string]interface{}{
						"Name":     "bitnami/redis",
						"Revision": "sha256:9c7f4a0958280a55a4337d74c22260bc338c26a0a2de493a8ad69dd73fd5c290",
					},
				},
			}
			return registry.ImageEntry{}, err
		},
	}
	factory := &mock.ClientFactory{Client: client}
	cache := &mem{}
	warmer := &Warmer{clientFactory: factory, cache: cache, burst: 10}

	logger := log.NewNopLogger()

	now := time.Now()
	redisRef, _ := image.ParseRef("bitnami/redis:5.0.2")
	repo := redisRef.Name
	warmer.warm(context.TODO(), now, logger, repo, registry.NoCredentials())

	registry := &Cache{Reader: cache}
	repoInfo, err := registry.GetImageRepositoryMetadata(repo)
	assert.NoError(t, err)

	assert.Len(t, repoInfo.Tags, 1)
	assert.Equal(t, tagWithMissingMetadata, repoInfo.Tags[0])
}

func TestRefreshDeadline(t *testing.T) {
	digest := "abc"
	warmer, cache := setup(t, &digest)
	logger := log.NewNopLogger()

	now0 := time.Now()
	warmer.warm(context.TODO(), now0, logger, repo, registry.NoCredentials())

	// We should see that there's an entry for the manifest, and that
	// it's set to be refreshed
	k := NewManifestKey(ref.CanonicalRef())
	_, deadline0, err := cache.GetKey(k)
	assert.NoError(t, err)
	assert.True(t, deadline0.After(now0))

	// Fast-forward to after the refresh deadline; check that the
	// entry is given a longer deadline
	now1 := deadline0.Add(time.Minute)
	warmer.warm(context.TODO(), now1, logger, repo, registry.NoCredentials())
	_, deadline1, err := cache.GetKey(k)
	assert.NoError(t, err)
	assert.True(t, deadline1.After(now1))
	assert.True(t, deadline0.Sub(now0) < deadline1.Sub(now1), "%s < %s", deadline0.Sub(now0), deadline1.Sub(now1))

	// Fast-forward again, check that a _differing_ manifest results
	// in a shorter deadline
	digest = "cba" // <-- means manifest points at a different image
	now2 := deadline1.Add(time.Minute)
	warmer.warm(context.TODO(), now2, logger, repo, registry.NoCredentials())
	_, deadline2, err := cache.GetKey(k)
	assert.NoError(t, err)
	assert.True(t, deadline1.Sub(now1) > deadline2.Sub(now2), "%s > %s", deadline1.Sub(now1), deadline2.Sub(now2))
}

func setup(t *testing.T, digest *string) (*Warmer, Client) {
	client := &mock.Client{
		TagsFn: func() ([]string, error) {
			println("asked for tags")
			return []string{"tag"}, nil
		},
		ManifestFn: func(tag string) (registry.ImageEntry, error) {
			println("asked for manifest", tag)
			if tag != "tag" {
				t.Errorf("remote client was asked for %q instead of %q", tag, "tag")
			}
			return registry.ImageEntry{
				Info: image.Info{
					ID:        ref,
					CreatedAt: time.Now(),
					Digest:    *digest,
				},
			}, nil
		},
	}
	factory := &mock.ClientFactory{Client: client}
	c := &mem{}
	warmer := &Warmer{clientFactory: factory, cache: c, burst: 10}
	return warmer, c
}
