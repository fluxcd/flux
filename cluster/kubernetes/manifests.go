package kubernetes

import (
	"io"
	"io/ioutil"
	"os"

	"github.com/weaveworks/flux"
	kresource "github.com/weaveworks/flux/cluster/kubernetes/resource"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/resource"
)

type Manifests struct {
}

// FindDefinedServices implementation in files.go

func (m *Manifests) LoadManifests(base, first string, rest ...string) (map[string]resource.Resource, error) {
	return kresource.Load(base, first, rest...)
}

func (m *Manifests) ParseManifests(allDefs []byte) (map[string]resource.Resource, error) {
	return kresource.ParseMultidoc(allDefs, "exported")
}

func (m *Manifests) UpdateImage(original io.Reader, id flux.ResourceID, container string, image image.Ref) (io.Reader, error) {
	return updatePodController(original, id, container, image)
}

// UpdatePolicies and ServicesWithPolicies in policies.go
