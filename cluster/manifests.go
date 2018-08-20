package cluster

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
)

type ManifestError struct {
	error
}

func ErrResourceNotFound(name string) error {
	return ManifestError{fmt.Errorf("manifest for resource %s not found under manifests path", name)}
}

// Manifests represents how a set of files are used as definitions of
// resources, e.g., in Kubernetes, YAML files describing Kubernetes
// resources.
type Manifests interface {
	// Update the image in a manifest's bytes to that given
	UpdateImage(def []byte, resourceID flux.ResourceID, container string, newImageID image.Ref) ([]byte, error)
	// Load all the resource manifests under the paths
	// given. `baseDir` is used to relativise the paths, which are
	// supplied as absolute paths to directories or files; at least
	// one path should be supplied, even if it is the same as `baseDir`.
	LoadManifests(baseDir string, paths []string) (map[string]resource.Resource, error)
	// Parse the manifests given in an exported blob
	ParseManifests([]byte) (map[string]resource.Resource, error)
	// UpdatePolicies modifies a manifest to apply the policy update specified
	UpdatePolicies([]byte, flux.ResourceID, policy.Update) ([]byte, error)
}

// UpdateManifest looks for the manifest for the identified resource,
// reads its contents, applies f(contents), and writes the results
// back to the file.
func UpdateManifest(m Manifests, root string, paths []string, id flux.ResourceID, f func(manifest []byte) ([]byte, error)) error {
	resources, err := m.LoadManifests(root, paths)
	if err != nil {
		return err
	}

	resource, ok := resources[id.String()]
	if !ok {
		return ErrResourceNotFound(id.String())
	}

	path := filepath.Join(root, resource.Source())
	def, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	newDef, err := f(def)
	if err != nil {
		return err
	}

	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, newDef, fi.Mode())
}
