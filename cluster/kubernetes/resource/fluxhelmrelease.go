package resource

import (
	"fmt"

	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/resource"
)

// ReleaseContainerName is the name used when flux interprets a
// FluxHelmRelease as having a container with an image, by virtue of
// having a `values` stanza with an image field:
//
// spec:
//   ...
//   values:
//     image: some/image:version
//
// The name refers to the source of the image value.
const ReleaseContainerName = "chart-image"

// CauseAnnotation is an annotation on a resource indicating that the
// cause of that resource (indirectly, via a Helm release) is a
// FluxHelmRelease. We use this rather than the `OwnerReference` type
// built into Kubernetes so that there are no garbage-collection
// implications. The value is expected to be a serialised
// `flux.ResourceID`.
const CauseAnnotation = "flux.weave.works/cause"

// FluxHelmRelease echoes the generated type for the custom resource
// definition. It's here so we can 1. get `baseObject` in there, and
// 3. control the YAML serialisation of fields, which we can't do
// (easily?) with the generated type.
type FluxHelmRelease struct {
	baseObject
	Spec struct {
		Values map[string]interface{}
	}
}

// Containers returns the containers that are defined in the
// FluxHelmRelease. At present, this assumes only one image in the
// Spec.Values, which is then named for the chart. If there is no such
// field, or it is not parseable as an image ref, no containers are
// returned.
func (fhr FluxHelmRelease) Containers() []resource.Container {
	values := fhr.Spec.Values
	if imgInfo, ok := values["image"]; ok {
		if imgInfoStr, ok := imgInfo.(string); ok {
			imageRef, err := image.ParseRef(imgInfoStr)
			if err == nil {
				return []resource.Container{
					{Name: ReleaseContainerName, Image: imageRef},
				}
			}
		}
	}
	return nil
}

// SetContainerImage mutates this resource by setting the `image`
// field of `values`, per the interpretation in `Containers` above. NB
// we can get away with a value-typed receiver because we set a map
// entry.
func (fhr FluxHelmRelease) SetContainerImage(container string, ref image.Ref) error {
	if container != ReleaseContainerName {
		return fmt.Errorf("container %q not in resource; expected %q by convention", container, ReleaseContainerName)
	}
	values := fhr.Spec.Values
	if _, ok := values["image"]; ok { // NB assume it's OK to replace whatever's there with a string
		values["image"] = ref.String()
		return nil
	}
	return fmt.Errorf("did not find 'image' field in resource")
}
