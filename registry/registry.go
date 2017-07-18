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
	"sort"
	"time"

	"github.com/docker/distribution/manifest/schema1"
	"github.com/go-kit/kit/log"
	dockerregistry "github.com/heroku/docker-registry-client/registry"
	"google.golang.org/appengine/memcache"

	"github.com/weaveworks/flux"
)

const (
	requestTimeout = 10 * time.Second
)

var (
	ErrNotCached = &flux.Missing{
		BaseError: &flux.BaseError{
			Err: memcache.ErrCacheMiss,
			Help: `Image not yet cached

It takes time to initially cache all the images. Please wait.

If you have waited for a long time, check the flux logs. Potential
reasons for the error are: no internet, no cache, error with the remote
repository.`,
		},
	}
)

// The Registry interface is a domain specific API to access container registries.
type Registry interface {
	GetRepository(id flux.ImageID) ([]flux.Image, error)
	GetImage(id flux.ImageID) (flux.Image, error)
}

type registry struct {
	factory     ClientFactory
	logger      log.Logger
	connections int
}

// NewClient creates a new registry registry, to use when fetching repositories.
// Behind the scenes the registry will call ClientFactory.ClientFor(...)
// when requesting an image. This will generate a Client to access the
// backend.
func NewRegistry(c ClientFactory, l log.Logger, connections int) Registry {
	return &registry{
		factory:     c,
		logger:      l,
		connections: connections,
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
	client, err := reg.factory.ClientFor(id.Host)
	if err != nil {
		return nil, err
	}

	tags, err := client.Tags(id)
	if err != nil {
		client.Cancel()
		// We have to test for equality of strings, rather than types,
		// because the ErrCacheMiss is a variable, not a constant.
		if err.Error() == memcache.ErrCacheMiss.Error() {
			return nil, ErrNotCached
		}
		return nil, err
	}

	// the hostlessImageName is canonicalised, in the sense that it
	// includes "library" as the org, if unqualified -- e.g.,
	// `library/nats`. We need that to fetch the tags etc. However, we
	// want the results to use the *actual* name of the images to be
	// as supplied, e.g., `nats`.
	return reg.tagsToRepository(client, id, tags)
}

// Get a single Image from the registry if it exists
func (reg *registry) GetImage(id flux.ImageID) (flux.Image, error) {
	client, err := reg.factory.ClientFor(id.Host)
	if err != nil {
		return flux.Image{}, err
	}
	img, err := client.Manifest(id)
	if err != nil {
		client.Cancel()
		// We have to test for equality of strings, rather than types,
		// because the ErrCacheMiss is a variable, not a constant.
		if err.Error() == memcache.ErrCacheMiss.Error() {
			return flux.Image{}, ErrNotCached
		}
		return flux.Image{}, err
	}
	return img, nil
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

	for i := 0; i < reg.connections; i++ {
		go func() {
			for tag := range toFetch {
				image, err := client.Manifest(id.WithNewTag(tag)) // Copy the imageID to avoid races
				if err != nil {
					if err.Error() == memcache.ErrCacheMiss.Error() {
						err = ErrNotCached
					} else {
						reg.logger.Log("registry-metadata-err", err)
					}
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
