package cluster

import (
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
)

// Manifests represents a set of files containing definitions of
// resources, e.g., in Kubernetes, YAML files describing Kubernetes
// resources.
type Manifests interface {
	// Load all the resource manifests under the paths
	// given. `baseDir` is used to relativise the paths, which are
	// supplied as absolute paths to directories or files; at least
	// one path should be supplied, even if it is the same as `baseDir`.
	LoadManifests(baseDir string, paths []string) (map[string]resource.Resource, error)
	// ParseManifest parses the content of a manifest and its source location into resources
	ParseManifest(def []byte, source string) (map[string]resource.Resource, error)
	// Set the image of a container in a manifest's bytes to that given
	SetWorkloadContainerImage(def []byte, resourceID flux.ResourceID, container string, newImageID image.Ref) ([]byte, error)
	// UpdatWorkloadPolicies modifies a manifest to apply the policy update specified
	UpdateWorkloadPolicies(def []byte, id flux.ResourceID, update policy.Update) ([]byte, error)
}
