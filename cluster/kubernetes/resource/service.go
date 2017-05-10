package resource

import (
	"fmt"

	"k8s.io/client-go/1.5/pkg/labels"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/resource"
)

// For reference:
// https://github.com/kubernetes/client-go/blob/master/pkg/api/v1/types.go#L2641

type Service struct {
	baseObject
	Spec ServiceSpec `yaml:"spec"`
}

func (o Service) ServiceIDs(all map[string]resource.Resource) []flux.ServiceID {
	// A service is part of its own service id
	ns := o.Meta.Namespace
	if ns == "" {
		ns = "default"
	}
	return []flux.ServiceID{flux.ServiceID(fmt.Sprintf("%s/%s", ns, o.Meta.Name))}
}

// Matches checks if this service's label selectors match the labels fo some
// deployment
func (o Service) Matches(l labels.Labels) bool {
	return o.Spec.Matches(l)
}

type ServiceSpec struct {
	Type     string            `yaml:"type"`
	Ports    []ServicePort     `yaml:"ports"`
	Selector map[string]string `yaml:"selector"`
}

func (s ServiceSpec) Matches(l labels.Labels) bool {
	return labels.SelectorFromSet(labels.Set(s.Selector)).Matches(l)
}

type ServicePort struct {
	Name       string `yaml:"name"`
	Protocol   string `yaml:"protocol"`
	Port       int32  `yaml:"port"`
	TargetPort string `yaml:"targetPort"`
	NodePort   int32  `yaml:"nodePort"`
}

// This is handy when we want to talk about flux.Services
func (s Service) ServiceID() flux.ServiceID {
	ns := s.Meta.Namespace
	if ns == "" {
		ns = "default"
	}
	return flux.MakeServiceID(ns, s.Meta.Name)
}
