package kubernetes

import (
	"strings"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/image"
)

// updateWorkload takes a YAML document stream (one or more YAML
// docs, as bytes), a resource ID referring to a controller, a
// container name, and the name of the new image that should be used
// for the container. It returns a new YAML stream where the image for
// the container has been replaced with the imageRef supplied.
func updateWorkload(in []byte, resource flux.ResourceID, container string, newImageID image.Ref) ([]byte, error) {
	namespace, kind, name := resource.Components()
	if _, ok := resourceKinds[strings.ToLower(kind)]; !ok {
		return nil, UpdateNotSupportedError(kind)
	}
	return (KubeYAML{}).Image(in, namespace, kind, name, container, newImageID.String())
}
