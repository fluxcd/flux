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
	// Try the simplest format first:
	// ```
	// values:
	//   image: 'repo/image:tag'
	// ```
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
	// Second most simple format:
	// ```
	// values:
	//   foo:
	//     image: repo/foo:tag
	//   bar:
	//     image: repo/bar:tag
	// ```
	var containers []resource.Container
	for k, v := range values {
		if v, ok := v.(map[interface{}]interface{}); ok {
			if imgInfo, ok := v["image"]; ok {
				if imgInfoStr, ok := imgInfo.(string); ok {
					imageRef, err := image.ParseRef(imgInfoStr)
					if err == nil {
						containers = append(containers, resource.Container{
							Name:  k,
							Image: imageRef,
						})
					}
				}
			}
		}
	}
	return containers
}

// SetContainerImage mutates this resource by setting the `image`
// field of `values`, per the interpretation in `Containers` above. NB
// we can get away with a value-typed receiver because we set a map
// entry.
func (fhr FluxHelmRelease) SetContainerImage(container string, ref image.Ref) error {
	values := fhr.Spec.Values
	if container == ReleaseContainerName {
		if existing, ok := values["image"]; ok {
			if _, ok := existing.(string); ok {
				values["image"] = ref.String()
				return nil
			}
			return fmt.Errorf("expected string value at .image, but it was not a string")
		}
		// if it isn't there, maybe it's the second format, and there
		// just happens to be an entry named the same as
		// `ReleaseContainerName`; so, fall through.
	}
	for k, v := range values {
		if k == container {
			if v, ok := v.(map[interface{}]interface{}); ok {
				if existing, ok := v["image"]; ok {
					if _, ok := existing.(string); ok {
						v["image"] = ref.String()
						return nil
					}
				}
			}
		}
	}
	return fmt.Errorf("expected string value at %s.image, but it is not present, or not a string", container)
}
