// Package registry provides domain abstractions over container registries.
package registry

import (
	"github.com/go-kit/kit/log"
	"github.com/weaveworks/flux"
	"sort"
	"time"
)

const (
	requestTimeout = 10 * time.Second
)

// Old registry interface. Used as an adaptor to the new class
type Client interface {
	GetRepository(repository string) ([]flux.ImageDescription, error)
}

type registryAdapter struct {
	r Registry
}

func NewClient(r Registry) Client {
	return &registryAdapter{
		r: r,
	}
}

func (a *registryAdapter) GetRepository(repository string) (res []flux.ImageDescription, err error) {
	img, err := ParseImage(repository, nil)
	if err != nil {
		return
	}
	images, err := a.r.GetRepository(img)
	if err != nil {
		return
	}
	res = make([]flux.ImageDescription, len(images))
	for i, im := range images {
		res[i] = flux.ImageDescription{
			ID:        flux.ParseImageID(im.FQN()),
			CreatedAt: im.CreatedAt(),
		}
	}
	return
}

// New registry interface
type Registry interface {
	GetRepository(repository Image) ([]Image, error)
	GetImage(repository Image) (Image, error)
}

// registry is a handle to a registry.
type registry struct {
	factory RemoteClientFactory
	Logger  log.Logger
	Metrics Metrics
}

// NewClient creates a new registry registry, to use when fetching repositories.
func NewRegistry(c RemoteClientFactory, l log.Logger, m Metrics) Registry {
	return &registry{
		factory: c,
		Logger:  l,
		Metrics: m,
	}
}

// GetRepository yields a repository matching the given name, if any exists.
// Repository may be of various forms, in which case omitted elements take
// assumed defaults.
//
//   helloworld             -> index.docker.io/library/helloworld
//   foo/helloworld         -> index.docker.io/foo/helloworld
//   quay.io/foo/helloworld -> quay.io/foo/helloworld
//
func (c *registry) GetRepository(img Image) (_ []Image, err error) {
	r, err := c.newRemote(img)
	if err != nil {
		return
	}

	tags, err := r.Tags(img)
	if err != nil {
		r.Cancel()
		return nil, err
	}

	// the hostlessImageName is canonicalised, in the sense that it
	// includes "library" as the org, if unqualified -- e.g.,
	// `library/nats`. We need that to fetch the tags etc. However, we
	// want the results to use the *actual* name of the images to be
	// as supplied, e.g., `nats`.
	return c.tagsToRepository(r, img, tags)
}

// Get a single image from the registry if it exists
func (c *registry) GetImage(img Image) (_ Image, err error) {
	r, err := c.newRemote(img)
	if err != nil {
		return
	}
	return r.Manifest(img)
}

func (c *registry) newRemote(img Image) (remote Remote, err error) {
	remote, err = c.factory.Create(img)
	if err != nil {
		return
	}
	remote = NewRemoteMonitoringMiddleware(c.Metrics)(remote)
	return
}

func (c *registry) tagsToRepository(remote Remote, img Image, tags []string) ([]Image, error) {
	// one way or another, we'll be finishing all requests
	defer remote.Cancel()

	type result struct {
		image Image
		err   error
	}

	fetched := make(chan result, len(tags))

	for _, tag := range tags {
		go func(t string) {
			i, err := remote.Manifest(img.WithTag(t))
			if err != nil {
				c.Logger.Log("registry-metadata-err", err)
			}
			fetched <- result{i, err}
		}(tag)
	}

	images := make([]Image, cap(fetched))
	for i := 0; i < cap(fetched); i++ {
		res := <-fetched
		if res.err != nil {
			return nil, res.err
		}
		images[i] = res.image
	}

	sort.Sort(byCreatedDesc(images))
	return images, nil
}

// -----

type byCreatedDesc []Image

func (is byCreatedDesc) Len() int      { return len(is) }
func (is byCreatedDesc) Swap(i, j int) { is[i], is[j] = is[j], is[i] }
func (is byCreatedDesc) Less(i, j int) bool {
	if is[i].CreatedAt() == nil {
		return true
	}
	if is[j].CreatedAt() == nil {
		return false
	}
	if is[i].CreatedAt().Equal(*is[j].CreatedAt()) {
		return is[i].FQN() < is[j].FQN()
	}
	return is[i].CreatedAt().After(*is[j].CreatedAt())
}
