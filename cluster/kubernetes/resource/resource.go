package resource

import (
	"strings"

	yaml "gopkg.in/yaml.v2"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
)

const (
	PolicyPrefix = "flux.weave.works/"
)

// -- unmarshaling code for specific object and field types

// struct to embed in objects, to provide default implementation
type baseObject struct {
	source string
	bytes  []byte
	Kind   string `yaml:"kind"`
	Meta   struct {
		Namespace   string            `yaml:"namespace"`
		Name        string            `yaml:"name"`
		Annotations map[string]string `yaml:"annotations,omitempty"`
	} `yaml:"metadata"`
}

func (o baseObject) ResourceID() flux.ResourceID {
	ns := o.Meta.Namespace
	if ns == "" {
		ns = "default"
	}
	return flux.MakeResourceID(ns, o.Kind, o.Meta.Name)
}

// It's useful for comparisons in tests to be able to remove the
// record of bytes
func (o *baseObject) debyte() {
	o.bytes = nil
}

func (o baseObject) Policy() policy.Set {
	set := policy.Set{}
	for k, v := range o.Meta.Annotations {
		if strings.HasPrefix(k, PolicyPrefix) && v == "true" {
			set = set.Add(policy.Policy(strings.TrimPrefix(k, PolicyPrefix)))
		}
	}
	return set
}

func (o baseObject) Source() string {
	return o.source
}

func (o baseObject) Bytes() []byte {
	return o.bytes
}

func unmarshalObject(source string, bytes []byte) (resource.Resource, error) {
	var base = baseObject{source: source, bytes: bytes}
	if err := yaml.Unmarshal(bytes, &base); err != nil {
		return nil, err
	}

	switch base.Kind {
	case "DaemonSet":
		var ds = DaemonSet{baseObject: base}
		if err := yaml.Unmarshal(bytes, &ds); err != nil {
			return nil, err
		}
		return &ds, nil
	case "Deployment":
		var dep = Deployment{baseObject: base}
		if err := yaml.Unmarshal(bytes, &dep); err != nil {
			return nil, err
		}
		return &dep, nil
	case "Namespace":
		var ns = Namespace{baseObject: base}
		if err := yaml.Unmarshal(bytes, &ns); err != nil {
			return nil, err
		}
		return &ns, nil
	case "StatefulSet":
		var ss = StatefulSet{baseObject: base}
		if err := yaml.Unmarshal(bytes, &ss); err != nil {
			return nil, err
		}
		return &ss, nil
		// The remainder are things we have to care about, but not
		// treat specially
	default:
		return &base, nil
	}
}

// For reference, the Kubernetes v1 types are in:
// https://github.com/kubernetes/client-go/blob/master/pkg/api/v1/types.go
