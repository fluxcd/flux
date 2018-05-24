package resource

import (
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/resource"
)

type DaemonSet struct {
	baseObject
	Spec struct {
		Template PodTemplate
	}
}

func (ds DaemonSet) Containers() []resource.Container {
	return ds.Spec.Template.Containers()
}

func (ds DaemonSet) SetContainerImage(container string, ref image.Ref) error {
	return ds.Spec.Template.SetContainerImage(container, ref)
}

var _ resource.Workload = DaemonSet{}
