package registry

import (
	"time"

	"github.com/docker/distribution/manifest/schema1"
	dockerregistry "github.com/heroku/docker-registry-client/registry"

	"github.com/weaveworks/flux/image"
)

const (
	requestTimeout = 10 * time.Second
)

type Registry interface {
	GetRepository(image.Name) ([]image.Info, error)
	GetImage(image.Ref) (image.Info, error)
}

// --- FIXME(michael): This is probably garbage?

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
