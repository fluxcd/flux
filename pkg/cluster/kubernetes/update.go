package kubernetes

import (
	"fmt"
	"strings"

	kresource "github.com/fluxcd/flux/pkg/cluster/kubernetes/resource"
	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/resource"
)

// updateWorkloadContainer takes a YAML document stream (one or more
// YAML docs, as bytes), a resource ID referring to a controller, a
// container name, and the name of the new image that should be used
// for the container. It returns a new YAML stream where the image for
// the container has been replaced with the imageRef supplied.
func updateWorkloadContainer(in []byte, resource resource.ID, container string, newImageID image.Ref) ([]byte, error) {
	namespace, kind, name := resource.Components()
	if _, ok := resourceKinds[strings.ToLower(kind)]; !ok {
		return nil, UpdateNotSupportedError(kind)
	}
	return (KubeYAML{}).Image(in, namespace, kind, name, container, newImageID.String())
}

// updateWorkloadImagePaths takes a YAML document stream (one or more
// YAML docs, as bytes), a resource ID referring to a HelmRelease,
// a ContainerImageMap, and the name of the new image that should be
// applied to the mapped paths. It returns a new YAML stream where
// the values of the paths have been replaced with the imageRef
// supplied.
func updateWorkloadImagePaths(in []byte,
	resource resource.ID, paths kresource.ContainerImageMap, newImageID image.Ref) ([]byte, error) {
	namespace, kind, name := resource.Components()
	// We only support HelmRelease resource kinds for now
	if kind != "helmrelease" {
		return nil, UpdateNotSupportedError(kind)
	}
	if m, ok := paths.MapImageRef(newImageID); ok {
		var args []string
		for k, v := range m {
			args = append(args, fmt.Sprintf("%s=%s", k, v))
		}
		return (KubeYAML{}).Set(in, namespace, kind, name, args...)
	}
	return nil, fmt.Errorf("failed to map paths %#v to %q for %q", paths, newImageID.String(), resource.String())
}
