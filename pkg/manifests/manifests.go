package manifests

import (
	"bytes"

	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/resource"
)

// Manifests represents a format for files or chunks of bytes
// containing definitions of resources, e.g., in Kubernetes, YAML
// files defining Kubernetes resources.
type Manifests interface {
	// Load all the resource manifests under the paths
	// given. `baseDir` is used to relativise the paths, which are
	// supplied as absolute paths to directories or files; at least
	// one path should be supplied, even if it is the same as `baseDir`.
	LoadManifests(baseDir string, paths []string) (map[string]resource.Resource, error)
	// ParseManifest parses the content of a collection of manifests, into resources
	ParseManifest(def []byte, source string) (map[string]resource.Resource, error)
	// Set the image of a container in a manifest's bytes to that given
	SetWorkloadContainerImage(def []byte, resourceID resource.ID, container string, newImageID image.Ref) ([]byte, error)
	// UpdateWorkloadPolicies modifies a manifest to apply the policy update specified
	UpdateWorkloadPolicies(def []byte, id resource.ID, update resource.PolicyUpdate) ([]byte, error)
	// CreateManifestPatch obtains a patch between the original and modified manifests
	CreateManifestPatch(originalManifests, modifiedManifests []byte, originalSource, modifiedSource string) ([]byte, error)
	// ApplyManifestPatch applies a manifest patch (obtained with CreateManifestPatch) returning the patched manifests
	ApplyManifestPatch(originalManifests, patchManifests []byte, originalSource, patchSource string) ([]byte, error)
	// AppendManifestToBuffer concatenates manifest bytes to a
	// (possibly empty) buffer of manifest bytes; the resulting bytes
	// should be parsable by `ParseManifest`.
	// TODO(michael) should really be an interface rather than `*bytes.Buffer`.
	AppendManifestToBuffer(manifest []byte, buffer *bytes.Buffer) error
}
