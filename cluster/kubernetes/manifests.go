package kubernetes

import (
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

func (m *Manifests) UpdateImage(path string, id flux.ResourceID, container string, image image.Ref) error {
	return updatePodController(path, id, container, image)
}

// UpdatePolicies and ServicesWithPolicies in policies.go

// ---

// updateManifest reads the contents at the path given, applies
// f(contents), and writes the results back to the file.
func updateManifest(path string, serviceID flux.ResourceID, f func(manifest []byte) ([]byte, error)) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}

	def, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	newDef, err := f(def)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, newDef, fi.Mode())
}
