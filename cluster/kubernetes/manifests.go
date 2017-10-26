package kubernetes

import (
	"github.com/weaveworks/flux"
	kresource "github.com/weaveworks/flux/cluster/kubernetes/resource"
	"github.com/weaveworks/flux/resource"
)

type Manifests struct {
}

// FindDefinedServices implementation in files.go

func (c *Manifests) LoadManifests(paths ...string) (map[string]resource.Resource, error) {
	return kresource.Load(paths...)
}

func (c *Manifests) ParseManifests(allDefs []byte) (map[string]resource.Resource, error) {
	return kresource.ParseMultidoc(allDefs, "exported")
}

func (c *Manifests) UpdateDefinition(def []byte, container string, image flux.ImageRef) ([]byte, error) {
	return updatePodController(def, container, image)
}

// UpdatePolicies and ServicesWithPolicies in policies.go
