package resource

import (
	"fmt"

	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/resource"
)

// FluxHelmRelease echoes the generated type for the custom resource
// definition. It's here so we can 1. get `baseObject` in there, and
// 3. control the YAML serialisation of fields, which we can't do
// (easily?) with the generated type.
type FluxHelmRelease struct {
	baseObject
	Spec struct {
		ChartGitPath string `yaml:"chartGitPath"`
		Values       map[string]interface{}
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
		imgInfoStr := imgInfo.(string)
		imageRef, err := image.ParseRef(imgInfoStr)
		if err != nil {
			return nil
		}
		return []resource.Container{
			{Name: fhr.Spec.ChartGitPath, Image: imageRef},
		}
	}

	return nil
}

// SetContainerImage mutates this resource by setting the `image`
// field of `values`, per the interpretation in `Containers` above. NB
// we can get away with a value-typed receiver because we set a map
// entry.
func (fhr FluxHelmRelease) SetContainerImage(container string, ref image.Ref) error {
	if container != fhr.Spec.ChartGitPath {
		return fmt.Errorf("container %q not in resource; found only container %q", container, fhr.Spec.ChartGitPath)
	}
	values := fhr.Spec.Values
	if _, ok := values["image"]; ok { // minor shortcut -- assume it's a string
		values["image"] = ref.String()
		return nil
	}
	return fmt.Errorf("did not find 'image' field in resource")
}
