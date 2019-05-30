package cluster

import (
	"bytes"
	"fmt"

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
	// UpdateWorkloadPolicies modifies a manifest to apply the policy update specified
	UpdateWorkloadPolicies(def []byte, id flux.ResourceID, update policy.Update) ([]byte, error)
	// CreateManifestPatch obtains a patch between the original and modified manifests
	CreateManifestPatch(originalManifests, modifiedManifests []byte, originalSource, modifiedSource string) ([]byte, error)
	// ApplyManifestPatch applies a manifest patch (obtained with CreateManifestDiff) returned the patched manifests
	ApplyManifestPatch(originalManifests, patchManifests []byte, originalSource, patchSource string) ([]byte, error)
}

func AppendManifestToBuffer(manifest []byte, buffer *bytes.Buffer) error {
	separator := "---\n"
	bytes := buffer.Bytes()
	if len(bytes) > 0 && bytes[len(bytes)-1] != '\n' {
		separator = "\n---\n"
	}
	if _, err := buffer.WriteString(separator); err != nil {
		return fmt.Errorf("cannot write to internal buffer: %s", err)
	}
	if _, err := buffer.Write(manifest); err != nil {
		return fmt.Errorf("cannot write to internal buffer: %s", err)
	}
	return nil
}
