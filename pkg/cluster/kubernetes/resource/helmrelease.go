package resource

import (
	"fmt"
	"github.com/Jeffail/gabs"
	"sort"
	"strings"

	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/resource"
)

const (
	// ReleaseContainerName is the name used when Flux interprets a
	// HelmRelease as having a container with an image, by virtue of
	// having a `values` stanza with an image field:
	//
	// spec:
	//   ...
	//   values:
	//     image: some/image:version
	//
	// The name refers to the source of the image value.
	ReleaseContainerName = "chart-image"

	// ImageBasePath is the default base path for image path mappings
	// in a HelmRelease resource.
	ImageBasePath = "spec.values."
	// ImageRegistryPrefix is the annotation key prefix for image
	// registry path mappings.
	ImageRegistryPrefix = "registry.fluxcd.io/"
	// ImageRepositoryPrefix is the annotation key prefix for image
	// repository path mappings.
	ImageRepositoryPrefix = "repository.fluxcd.io/"
	// ImageRepositoryPrefix is the annotation key prefix for image
	// tag path mappings.
	ImageTagPrefix = "tag.fluxcd.io/"
)

// ContainerImageMap holds the YAML dot notation paths to a
// container image.
type ContainerImageMap struct {
	BasePath   string
	Registry   string
	Repository string
	Tag        string
}

// RepositoryOnly returns if only the repository is defined.
func (c ContainerImageMap) RepositoryOnly() bool {
	return c.Repository != "" && c.Registry == "" && c.Tag == ""
}

// RegistryRepository returns if the repository and tag are
// defined, but the registry is not.
func (c ContainerImageMap) RepositoryTag() bool {
	return c.Repository != "" && c.Tag != "" && c.Registry == ""
}

// RegistryRepository returns if the registry and repository are
// defined, but the tag is not.
func (c ContainerImageMap) RegistryRepository() bool {
	return c.Registry != "" && c.Repository != "" && c.Tag == ""
}

// AllDefined returns if all image elements are defined.
func (c ContainerImageMap) AllDefined() bool {
	return c.Registry != "" && c.Repository != "" && c.Tag != ""
}

// GetRegistry returns the full registry path (with base path).
func (c ContainerImageMap) GetRegistry() string {
	if c.Registry == "" {
		return c.Registry
	}
	return fmt.Sprintf("%s%s", c.BasePath, c.Registry)
}

// GetRepository returns the full repository path (with base path).
func (c ContainerImageMap) GetRepository() string {
	if c.Repository == "" {
		return c.Repository
	}
	return fmt.Sprintf("%s%s", c.BasePath, c.Repository)
}

// GetTag returns the full tag path (with base path).
func (c ContainerImageMap) GetTag() string {
	if c.Tag == "" {
		return c.Tag
	}
	return fmt.Sprintf("%s%s", c.BasePath, c.Tag)
}

// MapImageRef maps the given imageRef to the dot notation paths
// ContainerImageMap holds. It needs at least an Repository to be able
// to compose the map, and takes the absence of the registry and/or tag
// paths into account to ensure all image elements (registry,
// repository, tag) are present in the returned map.
func (c ContainerImageMap) MapImageRef(image image.Ref) (map[string]string, bool) {
	m := make(map[string]string)
	switch {
	case c.AllDefined():
		m[c.GetRegistry()] = image.Domain
		m[c.GetRepository()] = image.Image
		m[c.GetTag()] = image.Tag
	case c.RegistryRepository():
		m[c.GetRegistry()] = image.Domain
		m[c.GetRepository()] = image.Image + ":" + image.Tag
	case c.RepositoryTag():
		m[c.GetRepository()] = image.Name.String()
		m[c.GetTag()] = image.Tag
	case c.RepositoryOnly():
		m[c.GetRepository()] = image.String()
	default:
		return m, false
	}
	return m, true
}

// HelmRelease echoes the generated type for the custom resource
// definition. It's here so we can 1. get `baseObject` in there, and
// 3. control the YAML serialisation of fields, which we can't do
// (easily?) with the generated type.
type HelmRelease struct {
	baseObject
	Spec struct {
		Values map[string]interface{}
	}
}

type ImageSetter func(image.Ref)

type imageAndSetter struct {
	image  image.Ref
	setter ImageSetter
}

// sorted_containers returns an array of container names in ascending
// order, except for `ReleaseContainerName`, which always goes first.
// We want a stable order to the containers we output, since things
// will jump around in API calls, or fail to verify, otherwise.
func sorted_containers(containers map[string]imageAndSetter) []string {
	var keys []string
	for k := range containers {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i] == ReleaseContainerName {
			return true
		}
		if keys[j] == ReleaseContainerName {
			return false
		}
		return keys[i] < keys[j]
	})
	return keys
}

// FindHelmReleaseContainers examines the Values from a
// HelmRelease (manifest, or cluster resource, or otherwise) and
// calls visit with each container name and image it finds, as well as
// procedure for changing the image value.
func FindHelmReleaseContainers(annotations map[string]string, values map[string]interface{},
	visit func(string, image.Ref, ImageSetter) error) {

	containers := make(map[string]imageAndSetter)
	// an image defined at the top-level is given a standard container name:
	if image, setter, ok := interpretAsContainer(stringMap(values)); ok {
		containers[ReleaseContainerName] = imageAndSetter{image, setter}
	}

	// an image as part of a field is treated as a "container" spec
	// named for the field:
	for k, v := range values {
		if image, setter, ok := interpret(v); ok {
			containers[k] = imageAndSetter{image, setter}
		}
	}

	// user mapped images, it will overwrite automagically interpreted
	// images with user defined ones:
	for k, v := range containerImageMappingsFromAnnotations(annotations) {
		if image, setter, ok := interpretMappedContainerImage(values, v); ok {
			containers[k] = imageAndSetter{image, setter}
		}
	}

	// sort the found containers by name, using the custom logic
	// defined in sorted_containers, so the calls to visit are
	// predictable:
	for _, k := range sorted_containers(containers) {
		visit(k, containers[k].image, containers[k].setter)
	}
}

// The following is some machinery for interpreting a
// HelmRelease's `values` field as defining images to be
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
		// container:
		//   image: 'repo/image:tag'
		fullImgStr := img

		var reggy bool
		// container:
		//   registry: registry.com
		//	 image: repo/foo
		if registry, ok := m.get("registry"); ok {
			if registryStr, ok := registry.(string); ok {
				reggy = true
				fullImgStr = registryStr + "/" + fullImgStr
			}
		}

		var taggy bool
		// container:
		//   image: repo/foo
		//   tag: v1
		if tag, ok := m.get("tag"); ok {
			if tagStr, ok := tag.(string); ok {
				taggy = true
				fullImgStr = fullImgStr + ":" + tagStr
			}
		}

		imageRef, err := image.ParseRef(fullImgStr)
		if err != nil {
			return image.Ref{}, nil, false
		}

		return imageRef, func(ref image.Ref) {
			switch {
			case reggy && taggy:
				m.set("registry", ref.Domain)
				m.set("image", ref.Image)
				m.set("tag", ref.Tag)
				return
			case reggy:
				m.set("registry", ref.Domain)
				m.set("image", ref.Name.Image+":"+ref.Tag)
			case taggy:
				m.set("image", ref.Name.String())
				m.set("tag", ref.Tag)
			default:
				m.set("image", ref.String())
			}
		}, true
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
	var imgRepo interface{}
	var ok bool
	if imgRepo, ok = m.get("repository"); !ok {
		return image.Ref{}, nil, false
	}

	// image:
	//   repository: repo/foo
	if imgStr, ok := imgRepo.(string); ok {
		fullImgStr := imgStr

		var reggy bool
		// image:
		//   registry: registry.com
		//   repository: repo/foo
		if registry, ok := m.get("registry"); ok {
			if registryStr, ok := registry.(string); ok {
				reggy = ok
				fullImgStr = registryStr + "/" + fullImgStr
			}
		}

		var taggy bool
		// image:
		//   repository: repo/foo
		//   tag: v1
		if tag, ok := m.get("tag"); ok {
			if tagStr, ok := tag.(string); ok {
				taggy = ok
				fullImgStr = fullImgStr + ":" + tagStr
			}
		}

		imageRef, err := image.ParseRef(fullImgStr)
		if err != nil {
			return image.Ref{}, nil, false
		}

		return imageRef, func(ref image.Ref) {
			switch {
			case reggy && taggy:
				m.set("registry", ref.Domain)
				m.set("repository", ref.Image)
				m.set("tag", ref.Tag)
				return
			case reggy:
				m.set("registry", ref.Domain)
				m.set("repository", ref.Name.Image+":"+ref.Tag)
			case taggy:
				m.set("repository", ref.Name.String())
				m.set("tag", ref.Tag)
			default:
				m.set("repository", ref.String())
			}
		}, true
	}

	return image.Ref{}, nil, false
}

// containerImageMappingsFromAnnotations collects yaml dot notation
// mappings of container images from the given annotations.
func containerImageMappingsFromAnnotations(annotations map[string]string) map[string]ContainerImageMap {
	cim := make(map[string]ContainerImageMap)
	for k, v := range annotations {
		switch {
		case strings.HasPrefix(k, ImageRegistryPrefix):
			container := strings.TrimPrefix(k, ImageRegistryPrefix)
			i, _ := cim[container]
			i.Registry = v
			cim[container] = i
		case strings.HasPrefix(k, ImageRepositoryPrefix):
			container := strings.TrimPrefix(k, ImageRepositoryPrefix)
			i, _ := cim[container]
			i.Repository = v
			cim[container] = i
		case strings.HasPrefix(k, ImageTagPrefix):
			container := strings.TrimPrefix(k, ImageTagPrefix)
			i, _ := cim[container]
			i.Tag = v
			cim[container] = i
		}
	}
	for k, _ := range cim {
		i, _ := cim[k]
		i.BasePath = ImageBasePath
		cim[k] = i
	}
	return cim
}

// interpretMappedContainerImage attempt to resolve the paths in the
// `ContainerImageMap` from the given values and tries to parse the
// resolved values into a valid `image.Ref`. It returns the
// `image.Ref`, an `ImageSetter` that is able to modify the image in
// the supplied values map, and a boolean that reflects if the
// interpretation was successful.
func interpretMappedContainerImage(values map[string]interface{}, cim ContainerImageMap) (image.Ref, ImageSetter, bool) {
	v, err := gabs.Consume(values)
	if err != nil {
		return image.Ref{}, nil, false
	}

	imageValue := v.Path(cim.Repository).Data()
	if img, ok := imageValue.(string); ok {
		switch {
		case cim.RepositoryOnly():
			if imgRef, err := image.ParseRef(img); err == nil {
				return imgRef, func(ref image.Ref) {
					v.SetP(ref.String(), cim.Repository)
				}, true
			}
		case cim.AllDefined():
			registryValue := v.Path(cim.Registry).Data()
			if reg, ok := registryValue.(string); ok {
				tagValue := v.Path(cim.Tag).Data()
				if tag, ok := tagValue.(string); ok {
					if imgRef, err := image.ParseRef(reg + "/" + img + ":" + tag); err == nil {
						return imgRef, func(ref image.Ref) {
							v.SetP(ref.Domain, cim.Registry)
							v.SetP(ref.Image, cim.Repository)
							v.SetP(ref.Tag, cim.Tag)
						}, true
					}
				}
			}
		case cim.RegistryRepository():
			registryValue := v.Path(cim.Registry).Data()
			if reg, ok := registryValue.(string); ok {
				if imgRef, err := image.ParseRef(reg + "/" + img); err == nil {
					return imgRef, func(ref image.Ref) {
						v.SetP(ref.Domain, cim.Registry)
						v.SetP(ref.Name.Image+":"+ref.Tag, cim.Repository)
					}, true
				}
			}
		case cim.RepositoryTag():
			tagValue := v.Path(cim.Tag).Data()
			if tag, ok := tagValue.(string); ok {
				if imgRef, err := image.ParseRef(img + ":" + tag); err == nil {
					return imgRef, func(ref image.Ref) {
						v.SetP(ref.Name.String(), cim.Repository)
						v.SetP(ref.Tag, cim.Tag)
					}, true
				}
			}
		}
	}

	return image.Ref{}, nil, false
}

// Containers returns the containers that are defined in the
// HelmRelease.
func (hr HelmRelease) Containers() []resource.Container {
	var containers []resource.Container
	addContainer := func(container string, image image.Ref, _ ImageSetter) error {
		containers = append(containers, resource.Container{
			Name:  container,
			Image: image,
		})
		return nil
	}
	FindHelmReleaseContainers(hr.Meta.Annotations, hr.Spec.Values, addContainer)
	return containers
}

// SetContainerImage mutates this resource by setting the `image`
// field of `values`, or a subvalue therein, per one of the
// interpretations in `FindHelmReleaseContainers` above. NB we can
// get away with a value-typed receiver because we set a map entry.
func (hr HelmRelease) SetContainerImage(container string, ref image.Ref) error {
	found := false
	imageSetter := func(name string, image image.Ref, setter ImageSetter) error {
		if container == name {
			setter(ref)
			found = true
		}
		return nil
	}
	FindHelmReleaseContainers(hr.Meta.Annotations, hr.Spec.Values, imageSetter)
	if !found {
		return fmt.Errorf("did not find container %s in HelmRelease", container)
	}
	return nil
}

// GetContainerImageMap returns the ContainerImageMap for a container,
// or an error if we were unable to interpret the mapping, or no mapping
// was found.
func (hr HelmRelease) GetContainerImageMap(container string) (ContainerImageMap, error) {
	cim := containerImageMappingsFromAnnotations(hr.Meta.Annotations)
	if c, ok := cim[container]; ok {
		if _, _, ok = interpretMappedContainerImage(hr.Spec.Values, c); ok {
			return c, nil
		}
	}
	return ContainerImageMap{}, fmt.Errorf("did not find image map for container %s in HelmRelease", container)
}
