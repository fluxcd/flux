package resource

import (
	"fmt"

	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/resource"
)

// Types that daemonsets, deployments, and other things have in
// common.

type ObjectMeta struct {
	Labels      map[string]string
	Annotations map[string]string
}

type PodTemplate struct {
	Metadata ObjectMeta
	Spec     PodSpec
}

func (t PodTemplate) Containers() []resource.Container {
	var result []resource.Container
	// FIXME(https://github.com/fluxcd/flux/issues/1269): account for possible errors (x2)
	for _, c := range t.Spec.Containers {
		im, _ := image.ParseRef(c.Image)
		result = append(result, resource.Container{Name: c.Name, Image: im})
	}
	for _, c := range t.Spec.InitContainers {
		im, _ := image.ParseRef(c.Image)
		result = append(result, resource.Container{Name: c.Name, Image: im})
	}
	return result
}

func (t PodTemplate) SetContainerImage(container string, ref image.Ref) error {
	for i, c := range t.Spec.Containers {
		if c.Name == container {
			t.Spec.Containers[i].Image = ref.String()
			return nil
		}
	}
	for i, c := range t.Spec.InitContainers {
		if c.Name == container {
			t.Spec.InitContainers[i].Image = ref.String()
			return nil
		}
	}
	return fmt.Errorf("container %q not found in workload", container)
}

type PodSpec struct {
	ImagePullSecrets []struct{ Name string }
	Volumes          []Volume
	Containers       []ContainerSpec
	InitContainers   []ContainerSpec `yaml:"initContainers"`
}

type Volume struct {
	Name   string
	Secret struct {
		SecretName string
	}
}

type ContainerSpec struct {
	Name  string
	Image string
	Args  Args
	Ports []ContainerPort
	Env   Env
}

type Args []string

type ContainerPort struct {
	ContainerPort int
	Name          string
}

type VolumeMount struct {
	Name      string
	MountPath string
	ReadOnly  bool
}

// Env is a bag of Name, Value pairs that are treated somewhat like a
// map.
type Env []EnvEntry

type EnvEntry struct {
	Name, Value string
}
