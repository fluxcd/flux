package registry

import (
	"sort"
	"time"

	"github.com/docker/distribution/manifest/schema1"
	"github.com/go-kit/kit/log"
	dockerregistry "github.com/heroku/docker-registry-client/registry"

	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/registry/cache"
)

const (
	requestTimeout = 10 * time.Second
)

type Registry interface {
	GetRepository(image.Name) ([]image.Info, error)
	GetImage(image.Ref) (image.Info, error)
}

type ClientRegistry struct {
	Factory ClientFactory
	Logger  log.Logger
}

// GetRepository returns the list of image manifests in an image
// repository (e.g,. at "quay.io/weaveworks/flux")
func (reg *ClientRegistry) GetRepository(id image.Name) ([]image.Info, error) {
	client, err := reg.Factory.ClientFor(id.Registry(), Credentials{})
	if err != nil {
		return nil, err
	}

	tags, err := client.Tags(id)
	if err != nil {
		client.Cancel()
		return nil, err
	}
	return reg.tagsToRepository(client, id, tags)
}

// GetImage gets the manifest of a specific image ref, from its
// registry.
func (reg *ClientRegistry) GetImage(id image.Ref) (image.Info, error) {
	client, err := reg.Factory.ClientFor(id.Registry(), Credentials{})
	if err != nil {
		return image.Info{}, err
	}
	img, err := client.Manifest(id)
	if err != nil {
		client.Cancel()
		return image.Info{}, err
	}
	return img, nil
}

func (reg *ClientRegistry) tagsToRepository(client Client, id image.Name, tags []string) ([]image.Info, error) {
	// one way or another, we'll be finishing all requests
	defer client.Cancel()

	type result struct {
		image image.Info
		err   error
	}

	toFetch := make(chan string, len(tags))
	fetched := make(chan result, len(tags))

	for i := 0; i < 100; i++ {
		go func() {
			for tag := range toFetch {
				image, err := client.Manifest(id.ToRef(tag))
				if err != nil {
					if err != cache.ErrNotCached {
						reg.Logger.Log("registry-metadata-err", err)
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

	images := make([]image.Info, cap(fetched))
	for i := 0; i < cap(fetched); i++ {
		res := <-fetched
		if res.err != nil {
			return nil, res.err
		}
		images[i] = res.image
	}

	sort.Sort(image.ByCreatedDesc(images))
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
