// Package registry provides domain abstractions over container registries.
package registry

import (
	"sort"
	"time"

	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux"
)

const (
	requestTimeout = 10 * time.Second
	maxConcurrency = 10 // Chosen arbitrarily
)

// The Registry interface is a domain specific API to access container registries.
type Registry interface {
	GetRepository(repository Repository) ([]flux.Image, error)
	GetImage(repository Repository, tag string) (flux.Image, error)
}

type registry struct {
	factory ClientFactory
	Logger  log.Logger
}

// NewClient creates a new registry registry, to use when fetching repositories.
func NewRegistry(c ClientFactory, l log.Logger) Registry {
	return &registry{
		factory: c,
		Logger:  l,
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
func (reg *registry) GetRepository(img Repository) (_ []flux.Image, err error) {
	rem, err := reg.newRemote(img)
	if err != nil {
		return
	}

	tags, err := rem.Tags(img)
	if err != nil {
		rem.Cancel()
		return nil, err
	}

	// the hostlessImageName is canonicalised, in the sense that it
	// includes "library" as the org, if unqualified -- e.g.,
	// `library/nats`. We need that to fetch the tags etc. However, we
	// want the results to use the *actual* name of the images to be
	// as supplied, e.g., `nats`.
	return reg.tagsToRepository(rem, img, tags)
}

// Get a single Image from the registry if it exists
func (reg *registry) GetImage(img Repository, tag string) (_ flux.Image, err error) {
	rem, err := reg.newRemote(img)
	if err != nil {
		return
	}
	return rem.Manifest(img, tag)
}

func (reg *registry) newRemote(img Repository) (rem Remote, err error) {
	client, cancel, err := reg.factory.ClientFor(img.Host())
	if err != nil {
		return
	}
	rem = newRemote(client, cancel)
	rem = NewInstrumentedRemote(rem)
	return
}

func (reg *registry) tagsToRepository(remote Remote, repository Repository, tags []string) ([]flux.Image, error) {
	// one way or another, we'll be finishing all requests
	defer remote.Cancel()

	type result struct {
		image flux.Image
		err   error
	}

	toFetch := make(chan string, len(tags))
	fetched := make(chan result, len(tags))

	for i := 0; i < maxConcurrency; i++ {
		go func() {
			for tag := range toFetch {
				image, err := remote.Manifest(repository, tag)
				if err != nil {
					reg.Logger.Log("registry-metadata-err", err)
				}
				fetched <- result{image, err}
			}
		}()
	}
	for _, tag := range tags {
		toFetch <- tag
	}
	close(toFetch)

	images := make([]flux.Image, cap(fetched))
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

type byCreatedDesc []flux.Image

func (is byCreatedDesc) Len() int      { return len(is) }
func (is byCreatedDesc) Swap(i, j int) { is[i], is[j] = is[j], is[i] }
func (is byCreatedDesc) Less(i, j int) bool {
	switch {
	case is[i].CreatedAt.IsZero():
		return true
	case is[j].CreatedAt.IsZero():
		return false
	case is[i].CreatedAt.Equal(is[j].CreatedAt):
		return is[i].ID.String() < is[j].ID.String()
	default:
		return is[i].CreatedAt.After(is[j].CreatedAt)
	}
}
