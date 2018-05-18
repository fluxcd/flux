package cluster

import (
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
)

// Manifests represents how a set of files are used as definitions of
// resources, e.g., in Kubernetes, YAML files describing Kubernetes
// resources.
type Manifests interface {
	// Given a directory with manifest files, find which files define
	// which services.
	// FIXME(michael): remove when redundant
	FindDefinedServices(path string) (map[flux.ResourceID][]string, error)
	// Update the image in the resource given to newImageID. We expect
	// to have obtained the resource by parsing manifest files;
	// however, the path is supplied explicitly, rather than obtained
	// from the resource, so that we can transplant changes from one
	// place to another.
	UpdateImage(abspath string, resourceID flux.ResourceID, container string, newImageID image.Ref) error
	// UpdatePolicies modifies a manifest to apply the policy update
	// specified. The path to the manifest file is given explicitly so
	// that changes may be applied to an arbitrary file (with the
	// expectation that the file has been determined to contain the
	// manifest somehow).
	UpdatePolicies(abspath string, id flux.ResourceID, update policy.Update) error
	// Load all the resource manifests under the path given. `baseDir`
	// is used to relativise the paths, which are supplied as absolute
	// paths to directories or files; at least one path must be
	// supplied.
	LoadManifests(baseDir, first string, rest ...string) (map[string]resource.Resource, error)
	// Parse the manifests given in an exported blob
	ParseManifests([]byte) (map[string]resource.Resource, error)
	// ServicesWithPolicies returns all services with their associated policies
	ServicesWithPolicies(path string) (policy.ResourceMap, error)
}
