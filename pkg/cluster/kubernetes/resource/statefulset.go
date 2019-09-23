package resource

import (
	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/resource"
)

type StatefulSet struct {
	baseObject
	Spec StatefulSetSpec
}

type StatefulSetSpec struct {
	Replicas int
	Template PodTemplate
}

func (ss StatefulSet) Containers() []resource.Container {
	return ss.Spec.Template.Containers()
}

func (ss StatefulSet) SetContainerImage(container string, ref image.Ref) error {
	return ss.Spec.Template.SetContainerImage(container, ref)
}

var _ resource.Workload = StatefulSet{}
