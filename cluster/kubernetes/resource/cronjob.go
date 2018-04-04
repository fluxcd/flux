package resource

import (
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
	}
}

func (c CronJob) Containers() []resource.Container {
	return c.Spec.JobTemplate.Spec.Template.Containers()
}
