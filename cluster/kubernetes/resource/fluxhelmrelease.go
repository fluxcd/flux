package resource

import (
	"fmt"
	"sort"

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

// The type we have to interpret as containers is a
// `map[string]interface{}`; and, we want a stable order to the
// containers we output, since things will jump around in API calls,
// or fail to verify, otherwise. Since we can't get them in the order
// they appear in the document, sort them.
func sorted_keys(values map[string]interface{}) []string {
	var keys []string
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// FindFluxHelmReleaseContainers examines the Values from a
// FluxHelmRelease (manifest, or cluster resource, or otherwise) and
// calls visit with each container name and image it finds, as well as
// procedure for changing the image value. It will return an error if
// it cannot interpret the values as specifying images, or if the
// `visit` function itself returns an error.
func FindFluxHelmReleaseContainers(values map[string]interface{}, visit func(string, image.Ref, ImageSetter) error) error {
	// an image defined at the top-level is given a standard container name:
	if image, setter, ok := interpretAsContainer(stringMap(values)); ok {
		visit(ReleaseContainerName, image, setter)
	}

	// an image as part of a field is treated as a "container" spec
	// named for the field:
	for _, k := range sorted_keys(values) {
		if image, setter, ok := interpret(values[k]); ok {
			visit(k, image, setter)
		}
	}
	return nil
}

// The following is some machinery for interpreting a
// FluxHelmRelease's `values` field as defining images to be
// interpolated into the chart templates.
//
// The top-level value is a map[string]interface{}, but beneath that,
// we get maps in two varieties: from a YAML (i.e., a file), they are
// `map[interface{}]interface{}`, and from JSON (i.e., Kubernetes API)
// they are a `map[string]interface{}`. To conflate them, here's an
// interface for maps:

type mapper interface {
	get(string) (interface{}, bool)
	set(string, interface{})
}

type stringMap map[string]interface{}
type anyMap map[interface{}]interface{}

func (m stringMap) get(k string) (interface{}, bool) { v, ok := m[k]; return v, ok }
func (m stringMap) set(k string, v interface{})      { m[k] = v }

func (m anyMap) get(k string) (interface{}, bool) { v, ok := m[k]; return v, ok }
func (m anyMap) set(k string, v interface{})      { m[k] = v }

// interpret gets a value which may contain a description of an image.
func interpret(values interface{}) (image.Ref, ImageSetter, bool) {
	switch m := values.(type) {
	case map[string]interface{}:
		return interpretAsContainer(stringMap(m))
	case map[interface{}]interface{}:
		return interpretAsContainer(anyMap(m))
	}
	return image.Ref{}, nil, false
}

// interpretAsContainer takes a `mapper` value that may _contain_ an
// image, and attempts to interpret it.
func interpretAsContainer(m mapper) (image.Ref, ImageSetter, bool) {
	imageValue, ok := m.get("image")
	if !ok {
		return image.Ref{}, nil, false
	}
	switch img := imageValue.(type) {
	case string:
		// ```
		// container:
		//   image: 'repo/image:tag'
		// ```
		imageRef, err := image.ParseRef(img)
		if err == nil {
			var taggy bool
			if tag, ok := m.get("tag"); ok {
				//   container:
				//     image: repo/foo
				//     tag: v1
				if tagStr, ok := tag.(string); ok {
					taggy = true
					imageRef.Tag = tagStr
				}
			}
			return imageRef, func(ref image.Ref) {
				if taggy {
					m.set("image", ref.Name.String())
					m.set("tag", ref.Tag)
					return
				}
				m.set("image", ref.String())
			}, true
		}
	case map[string]interface{}:
		return interpretAsImage(stringMap(img))
	case map[interface{}]interface{}:
		return interpretAsImage(anyMap(img))
	}
	return image.Ref{}, nil, false
}

// interpretAsImage takes a `mapper` value that may represent an
// image, and attempts to interpret it.
func interpretAsImage(m mapper) (image.Ref, ImageSetter, bool) {
	var imgRepo, imgTag interface{}
	var ok bool
	if imgRepo, ok = m.get("repository"); !ok {
		return image.Ref{}, nil, false
	}

	if imgTag, ok = m.get("tag"); !ok {
		return image.Ref{}, nil, false
	}

	if imgStr, ok := imgRepo.(string); ok {
		if tagStr, ok := imgTag.(string); ok {
			//    container:
			//      image:
			//        repository: repo/bar
			//        tag: v1
			imgRef, err := image.ParseRef(imgStr + ":" + tagStr)
			if err == nil {
				return imgRef, func(ref image.Ref) {
					m.set("repository", ref.Name.String())
					m.set("tag", ref.Tag)
				}, true
			}
		}
	}
	return image.Ref{}, nil, false
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
