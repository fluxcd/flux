package resource

import (
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/resource"
)

type CronJob struct {
	baseObject
	Spec CronJobSpec
}

type CronJobSpec struct {
	JobTemplate struct {
		Spec struct {
			Template PodTemplate
		}
	} `yaml:"jobTemplate"`
}

func (c CronJob) Containers() []resource.Container {
	return c.Spec.JobTemplate.Spec.Template.Containers()
}

func (c CronJob) SetContainerImage(container string, ref image.Ref) error {
	return c.Spec.JobTemplate.Spec.Template.SetContainerImage(container, ref)
}

var _ resource.Workload = CronJob{}
