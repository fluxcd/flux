package resource

import (
	"fmt"
	"io"

	"github.com/weaveworks/flux"
	ifv1 "github.com/weaveworks/flux/apis/helm.integrations.flux.weave.works/v1alpha2"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/resource"
	apiv1 "k8s.io/api/core/v1"
)

type FluxHelmRelease struct {
	baseObject
	Spec ifv1.FluxHelmReleaseSpec
}

func (fhr FluxHelmRelease) Containers() []resource.Container {
	containers, err := fhr.createFluxFHRContainers()
	if err != nil {
		// log ?
	}
	return containers
}

// CreateK8sContainers creates a list of k8s containers as
func CreateK8sFHRContainers(spec ifv1.FluxHelmReleaseSpec) []apiv1.Container {
	containers := []apiv1.Container{}

	values := spec.Values
	if len(values) == 0 {
		return containers
	}

	imgInfo, ok := values["image"]

	// image info appears on the top level, so is associated directly with the chart
	if ok {
		imgInfoStr, ok := imgInfo.(string)
		if !ok {
			return containers
		}

		cont := apiv1.Container{Name: spec.ChartGitPath, Image: imgInfoStr}
		containers = append(containers, cont)

		return containers
	}

	return []apiv1.Container{}
}

func TryFHRUpdate(def []byte, resourceID flux.ResourceID, container string, newImage image.Ref, out io.Writer) error {
	fmt.Println("FAKE Updating image tag info for FHR special")
	fmt.Println("=========================================")
	fmt.Println("\t\t*** in tryFHRUpdate")
	fmt.Printf("\t\t*** container: %s\n", container)
	fmt.Printf("\t\t*** newImage: %+v\n", newImage)

	fmt.Println("Updating image tag info for FHR special")
	fmt.Println("=========================================")

	return nil
}

// assumes only one image in the Spec.Values
func (fhr FluxHelmRelease) createFluxFHRContainers() ([]resource.Container, error) {
	values := fhr.Spec.Values
	containers := []resource.Container{}

	if len(values) == 0 {
		return containers, nil
	}

	imgInfo, ok := values["image"]

	// image info appears on the top level, so is associated directly with the chart
	if ok {
		imgInfoStr := imgInfo.(string)
		imageRef, err := image.ParseRef(imgInfoStr)
		if err != nil {
			return containers, err
		}
		containers = append(containers, resource.Container{Name: fhr.Spec.ChartGitPath, Image: imageRef})
		return containers, nil
	}

	return []resource.Container{}, nil
}
