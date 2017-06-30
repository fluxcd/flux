// Package registry provides domain abstractions over container registries.
// The aim is that the user only ever sees the registry information that
// has been cached. A separate process is responsible for ensuring the
// cache is up to date. The benefit of this is that we can rate limit
// the requests to prevent rate limiting on the remote side without
// affecting the UX. To the user, repository information will appear to
// be returned "quickly"
//
// This means that the cache is now a flux requirement.
package registry

import (
	"github.com/docker/distribution/manifest/schema1"
	"github.com/go-kit/kit/log"
	dockerregistry "github.com/heroku/docker-registry-client/registry"
	"github.com/weaveworks/flux"
	"sort"
	"time"
)

const (
	requestTimeout = 10 * time.Second
	maxConcurrency = 10 // Chosen arbitrarily
)

// The Registry interface is a domain specific API to access container registries.
type Registry interface {
	GetRepository(id flux.ImageID) ([]flux.Image, error)
	GetImage(id flux.ImageID, tag string) (flux.Image, error)
}

type registry struct {
	factory ClientFactory
	Logger  log.Logger
}

// NewClient creates a new registry registry, to use when fetching repositories.
// Behind the scenes the registry will call ClientFactory.ClientFor(...)
// when requesting an image. This will generate a Client to access the
// backend.
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
func (reg *registry) GetRepository(id flux.ImageID) ([]flux.Image, error) {
	rem, err := reg.newClient(id)
	if err != nil {
		return nil, err
	}

	tags, err := rem.Tags(id)
	if err != nil {
		rem.Cancel()
		return nil, err
	}

	// the hostlessImageName is canonicalised, in the sense that it
	// includes "library" as the org, if unqualified -- e.g.,
	// `library/nats`. We need that to fetch the tags etc. However, we
	// want the results to use the *actual* name of the images to be
	// as supplied, e.g., `nats`.
	return reg.tagsToRepository(rem, id, tags)
}

// Get a single Image from the registry if it exists
func (reg *registry) GetImage(id flux.ImageID, tag string) (_ flux.Image, err error) {
	rem, err := reg.newClient(id)
	if err != nil {
		return
	}
	return rem.Manifest(id, tag)
}

func (reg *registry) newClient(id flux.ImageID) (Client, error) {
	client, err := reg.factory.ClientFor(id.Host)
	if err != nil {
		return nil, err
	}
	client = NewInstrumentedClient(client)
	return client, nil
}

func (reg *registry) tagsToRepository(client Client, id flux.ImageID, tags []string) ([]flux.Image, error) {
	// one way or another, we'll be finishing all requests
	defer client.Cancel()

	type result struct {
		image flux.Image
		err   error
	}

	toFetch := make(chan string, len(tags))
	fetched := make(chan result, len(tags))

	for i := 0; i < maxConcurrency; i++ {
		go func() {
			for tag := range toFetch {
				image, err := client.Manifest(id, tag)
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

	sort.Sort(flux.ByCreatedDesc(images))
	return images, nil
}

// ---

// This is an interface that represents the heroku docker registry library
type HerokuRegistryLibrary interface {
	Tags(repository string) (tags []string, err error)
	Manifest(repository, reference string) ([]schema1.History, error)
}

// ---

// Convert between types. dockerregistry returns the *same* type but from a
// vendored library. Because golang doesn't like to apply interfaces to a
// vendored type, we have to provide an adaptor to isolate it.
type herokuManifestAdaptor struct {
	*dockerregistry.Registry
}

func (h herokuManifestAdaptor) Manifest(repository, reference string) ([]schema1.History, error) {
	manifest, err := h.Registry.Manifest(repository, reference)
	if err != nil || manifest == nil {
		return nil, err
	}
	var result []schema1.History
	for _, item := range manifest.History {
		result = append(result, schema1.History{item.V1Compatibility})
	}
	return result, err
}
