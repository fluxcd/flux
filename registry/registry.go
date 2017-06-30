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
	GetRepository(repository Repository) ([]flux.Image, error)
	GetImage(repository Repository, tag string) (flux.Image, error)
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
func (reg *registry) GetRepository(img Repository) ([]flux.Image, error) {
	rem, err := reg.newClient(img)
	if err != nil {
		return nil, err
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
	rem, err := reg.newClient(img)
	if err != nil {
		return
	}
	return rem.Manifest(img, tag)
}

func (reg *registry) newClient(img Repository) (Client, error) {
	client, err := reg.factory.ClientFor(img.Host())
	if err != nil {
		return nil, err
	}
	client = NewInstrumentedClient(client)
	return client, nil
}

func (reg *registry) tagsToRepository(client Client, repository Repository, tags []string) ([]flux.Image, error) {
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
				image, err := client.Manifest(repository, tag)
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

// Repository represents a full image address, including host.
// TODO: This could probably be merged into flux.Image.
type Repository struct {
	img flux.Image // Internally we use an image to store data
}

func RepositoryFromImage(img flux.Image) Repository {
	return Repository{
		img: img,
	}
}

func ParseRepository(imgStr string) (Repository, error) {
	i, err := flux.ParseImage(imgStr, time.Time{})
	if err != nil {
		return Repository{}, err
	}
	return Repository{
		img: i,
	}, nil
}

func (r Repository) NamespaceImage() string {
	return r.img.ID.NamespaceImage()
}

func (r Repository) Host() string {
	return r.img.ID.Host
}

func (r Repository) String() string {
	return r.img.ID.HostNamespaceImage()
}

func (r Repository) ToImage(tag string) flux.Image {
	newImage := r.img
	newImage.ID.Tag = tag
	return newImage
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
