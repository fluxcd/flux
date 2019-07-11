package kubernetes

import (
	"strings"

	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/resource"
)

// updateWorkload takes a YAML document stream (one or more YAML
// docs, as bytes), a resource ID referring to a controller, a
// container name, and the name of the new image that should be used
// for the container. It returns a new YAML stream where the image for
// the container has been replaced with the imageRef supplied.
func updateWorkload(in []byte, resource resource.ID, container string, newImageID image.Ref) ([]byte, error) {
	namespace, kind, name := resource.Components()
	if _, ok := resourceKinds[strings.ToLower(kind)]; !ok {
		return nil, UpdateNotSupportedError(kind)
	}
	return (KubeYAML{}).Image(in, namespace, kind, name, container, newImageID.String())
}
