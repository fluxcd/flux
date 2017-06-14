package resource

import (
	"k8s.io/client-go/1.5/pkg/labels"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/resource"
)

type Deployment struct {
	baseObject
	Spec DeploymentSpec
}

func (o Deployment) ServiceIDs(all map[string]resource.Resource) []flux.ServiceID {
	found := flux.ServiceIDMap{}
	// Look through all for any matching services
	for _, r := range all {
		s, ok := r.(*Service)
		if ok && s.Matches(labels.Set(o.Spec.Template.Metadata.Labels)) {
			found.Add(s.ServiceIDs(all))
		}
	}

	return found.ToSlice()
}

type DeploymentSpec struct {
	Replicas int
	Template PodTemplate
}
