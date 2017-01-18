// Package registry provides domain abstractions over container registries.
package registry

import (
	"github.com/go-kit/kit/log"
	"sort"
	"time"
)

const (
	requestTimeout = 10 * time.Second
)

// New registry interface
type Registry interface {
	GetRepository(repository Repository) ([]Image, error)
	GetImage(repository Repository, tag string) (Image, error)
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
func (c *registry) GetRepository(img Repository) (_ []Image, err error) {
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

// Get a single Image from the registry if it exists
func (c *registry) GetImage(img Repository, tag string) (_ Image, err error) {
	r, err := c.newRemote(img)
	if err != nil {
		return
	}
	return r.Manifest(img, tag)
}

func (c *registry) newRemote(img Repository) (remote Remote, err error) {
	remote, err = c.factory.CreateFor(img.Host())
	if err != nil {
		return
	}
	remote = NewInstrumentedRemote(c.Metrics)(remote)
	return
}

func (c *registry) tagsToRepository(remote Remote, repository Repository, tags []string) ([]Image, error) {
	// one way or another, we'll be finishing all requests
	defer remote.Cancel()

	type result struct {
		image Image
		err   error
	}

	fetched := make(chan result, len(tags))

	for _, tag := range tags {
		go func(t string) {
			i, err := remote.Manifest(repository, t)
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
	if is[i].CreatedAt == nil {
		return true
	}
	if is[j].CreatedAt == nil {
		return false
	}
	if is[i].CreatedAt.Equal(*is[j].CreatedAt) {
		return is[i].String() < is[j].String()
	}
	return is[i].CreatedAt.After(*is[j].CreatedAt)
}
