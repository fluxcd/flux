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

type ImageSetter func(image.Ref)

// FindFluxHelmReleaseContainers examines the Values from a
// FluxHelmRelease (manifest, or cluster resource, or otherwise) and
// calls visit with each container name and image it finds, as well as
// procedure for changing the image value. It will return an error if
// it cannot interpret the values as specifying images, or if the
// `visit` function itself returns an error.
func FindFluxHelmReleaseContainers(values map[string]interface{}, visit func(string, image.Ref, ImageSetter) error) error {
	// Try the simplest format first:
	// ```
	// values:
	//   image: 'repo/image:tag'
	// ```
	if imgInfo, ok := values["image"]; ok {
		if imgInfoStr, ok := imgInfo.(string); ok {
			imageRef, err := image.ParseRef(imgInfoStr)
			if err == nil {
				return visit(ReleaseContainerName, imageRef, func(ref image.Ref) {
					values["image"] = ref.String()
				})
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
	for k, v := range values {
		var imgInfo interface{}
		var ok bool
		var setter ImageSetter
		// From a YAML (i.e., a file), it's a
		// `map[interface{}]interface{}`, and from JSON (i.e.,
		// Kubernetes API) it's a `map[string]interface{}`.
		switch m := v.(type) {
		case map[string]interface{}:
			imgInfo, ok = m["image"]
			setter = func(ref image.Ref) {
				m["image"] = ref.String()
			}
		case map[interface{}]interface{}:
			imgInfo, ok = m["image"]
			setter = func(ref image.Ref) {
				m["image"] = ref.String()
			}
		}
		if ok {
			if imgInfoStr, ok := imgInfo.(string); ok {
				imageRef, err := image.ParseRef(imgInfoStr)
				if err == nil {
					err = visit(k, imageRef, setter)
				}
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Containers returns the containers that are defined in the
// FluxHelmRelease.
func (fhr FluxHelmRelease) Containers() []resource.Container {
	var containers []resource.Container
	// If there's an error in interpreting, return what we have.
	_ = FindFluxHelmReleaseContainers(fhr.Spec.Values, func(container string, image image.Ref, _ ImageSetter) error {
		containers = append(containers, resource.Container{
			Name:  container,
			Image: image,
		})
		return nil
	})
	return containers
}

// SetContainerImage mutates this resource by setting the `image`
// field of `values`, or a subvalue therein, per one of the
// interpretations in `FindFluxHelmReleaseContainers` above. NB we can
// get away with a value-typed receiver because we set a map entry.
func (fhr FluxHelmRelease) SetContainerImage(container string, ref image.Ref) error {
	found := false
	if err := FindFluxHelmReleaseContainers(fhr.Spec.Values, func(name string, image image.Ref, setter ImageSetter) error {
		if container == name {
			setter(ref)
			found = true
		}
		return nil
	}); err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("did not find container %s in FluxHelmRelease", container)
	}
	return nil
}
