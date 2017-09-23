package kubernetes

import (
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"

	"github.com/weaveworks/flux/cluster/kubernetes/resource"
)

// FindDefinedServices finds all the services defined under the
// directory given, and returns a map of service IDs (from its
// specified namespace and name) to the paths of resource definition
// files.
func (c *Manifests) FindDefinedServices(path string) (map[flux.ServiceID][]string, error) {
	objects, err := resource.Load(path)
	if err != nil {
		return nil, errors.Wrap(err, "loading resources")
	}

	type template struct {
		source    string
		namespace string
		*resource.PodTemplate
	}

	var (
		result    = map[flux.ServiceID][]string{}
		services  []*resource.Service
		templates []template
	)

	for _, obj := range objects {
		switch res := obj.(type) {
		case *resource.Service:
			services = append(services, res)
			for _, template := range templates {
				if res.Meta.Namespace == template.namespace && matches(res, template.PodTemplate) {
					sid := res.ServiceID()
					result[sid] = appendIfMissing(result[sid], template.source)
				}
			}
		case *resource.Deployment:
			source := res.Source()
			templates = append(templates, template{source, res.Meta.Namespace, &res.Spec.Template})
			for _, service := range services {
				if res.Meta.Namespace == service.Meta.Namespace && matches(service, &res.Spec.Template) {
					sid := service.ServiceID()
					result[sid] = appendIfMissing(result[sid], source)
				}
			}
		}
	}
	return result, nil
}

func matches(s *resource.Service, t *resource.PodTemplate) bool {
	labels := t.Metadata.Labels
	selector := s.Spec.Selector
	// A nil selector matches nothing
	if selector == nil {
		return false
	}

	// otherwise, each label in the selector must have a match in the
	// pod
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

func appendIfMissing(slice []string, i string) []string {
	for _, v := range slice {
		if v == i {
			return slice
		}
	}
	return append(slice, i)
}
