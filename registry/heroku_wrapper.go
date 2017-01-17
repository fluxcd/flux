package registry

import (
	"github.com/docker/distribution/manifest/schema1"
	dockerregistry "github.com/heroku/docker-registry-client/registry"
)

type herokuWrapper struct {
	*dockerregistry.Registry
}

// Convert between types. dockerregistry returns the *same* type but from a
// vendored library so go freaks out. Eugh.
// TODO: The only thing we care about here for now is history. Frankly it might
// be easier to convert it to JSON and back.
func (h herokuWrapper) Manifest(repository, reference string) (*schema1.SignedManifest, error) {
	manifest, err := h.Registry.Manifest(repository, reference)
	result := &schema1.SignedManifest{}
	for _, item := range manifest.History {
		result.Manifest.History = append(result.Manifest.History, schema1.History{item.V1Compatibility})
	}
	return result, err
}
