package resource

import (
	"fmt"

	"k8s.io/client-go/1.5/pkg/labels"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/resource"
)

type Deployment struct {
	baseObject
	Spec DeploymentSpec
}

func (o Deployment) ServiceIDs(all map[string]resource.Resource) []flux.ServiceID {
	found := flux.ServiceIDSet{}

	// Add the base service for this deployment
	// TODO: Is this even the right thing to do?
	ns := o.Meta.Namespace
	if ns == "" {
		ns = "default"
	}
	found.Add([]flux.ServiceID{flux.ServiceID(fmt.Sprintf("%s/%s", ns, o.Meta.Name))})

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
