package resource

import (
	"fmt"

	yaml "gopkg.in/yaml.v2"

	"github.com/weaveworks/flux/resource"
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

func (o baseObject) ResourceID() string {
	ns := o.Meta.Namespace
	if ns == "" {
		ns = "default"
	}
	return fmt.Sprintf("%s %s/%s", o.Kind, ns, o.Meta.Name)
}

// It's useful for comparisons in tests to be able to remove the
// record of bytes
func (o *baseObject) debyte() {
	o.bytes = nil
}

func (o baseObject) Annotations() map[string]string {
	return o.Meta.Annotations
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
	case "Deployment":
		var dep = Deployment{baseObject: base}
		if err := yaml.Unmarshal(bytes, &dep); err != nil {
			return nil, err
		}
		return &dep, nil
	case "Service":
		var svc = Service{baseObject: base}
		if err := yaml.Unmarshal(bytes, &svc); err != nil {
			return nil, err
		}
		return &svc, nil
	case "Namespace":
		var ns = Namespace{baseObject: base}
		if err := yaml.Unmarshal(bytes, &ns); err != nil {
			return nil, err
		}
		return &ns, nil
		// The remainder are things we have to care about, but not
		// treat specially
	default:
		return &base, nil
	}
}

// For reference, the Kubernetes v1 types are in:
// https://github.com/kubernetes/client-go/blob/master/pkg/api/v1/types.go
