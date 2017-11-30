package cache

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/registry"
	"github.com/weaveworks/flux/registry/mock"
)

type mem struct {
	kv map[string][]byte
	mx sync.Mutex
}

func (c *mem) SetKey(k Keyer, v []byte) error {
	c.mx.Lock()
	defer c.mx.Unlock()
	if c.kv == nil {
		c.kv = make(map[string][]byte)
	}
	c.kv[k.Key()] = v
	return nil
}

func (c *mem) GetKey(k Keyer) ([]byte, time.Time, error) {
	c.mx.Lock()
	defer c.mx.Unlock()
	if c.kv == nil {
		c.kv = make(map[string][]byte)
	}

	if v, ok := c.kv[k.Key()]; ok {
		return v, time.Now().Add(time.Hour), nil
	}
	return nil, time.Time{}, ErrNotCached
}

// WarmTest effectively checks that the cache.Warmer and
// cache.Registry work together as intended: that is, if you ask the
// warmer to fetch information, the cached gets populated, and the
// Registry implementation will see it.
func TestWarm(t *testing.T) {
	ref, _ := image.ParseRef("example.com/path/image:tag")
	repo := ref.Name

	client := &mock.Client{
		TagsFn: func() ([]string, error) {
			return []string{"tag"}, nil
		},
		ManifestFn: func(tag string) (image.Info, error) {
			if tag != "tag" {
				t.Errorf("remote client was asked for %q instead of %q", tag, "tag")
			}
			return image.Info{
				ID:        ref,
				CreatedAt: time.Now(),
			}, nil
		},
	}
	factory := &mock.ClientFactory{Client: client}
	c := &mem{}
	warmer := &Warmer{Logger: log.NewNopLogger(), ClientFactory: factory, Cache: c, Burst: 10}
	warmer.warm(context.TODO(), repo, registry.NoCredentials())

	registry := &Cache{Reader: c}
	repoInfo, err := registry.GetRepository(ref.Name)
	if err != nil {
		t.Error(err)
	}
	// Otherwise, we should get what we put in ...
	if len(repoInfo) != 1 {
		t.Errorf("expected an image.Info item; got %#v", repoInfo)
	} else {
		if got := repoInfo[0].ID.String(); got != ref.String() {
			t.Errorf("expected image %q from registry cache; got %q", ref.String(), got)
		}
	}
}
