package resource

import (
	"errors"
	"fmt"

	yaml "gopkg.in/yaml.v2"
)

// For the minute we just care about
type Resource interface {
	ResourceID() string // name, to correlate with what's in the cluster
	Source() string     // where did this come from (informational)
	Bytes() []byte      // the definition, for sending to platform.Sync
}

// -- unmarshaling code for specific object and field types

// struct to embed in objects, to provide default implementation
type baseObject struct {
	source string
	bytes  []byte
	Kind   string `yaml:"kind"`
	Meta   struct {
		Namespace string `yaml:"namespace"`
		Name      string `yaml:"name"`
	} `yaml:"metadata"`
}

func (o baseObject) ResourceID() string {
	ns := o.Meta.Namespace
	if ns == "" {
		ns = "default"
	}
	return fmt.Sprintf("%s %s/%s", o.Kind, ns, o.Meta.Name)
}

func (o baseObject) Source() string {
	return o.source
}

func (o baseObject) Bytes() []byte {
	return o.bytes
}

func unmarshalObject(source string, bytes []byte) (Resource, error) {
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
	}

	// FIXME what about other kinds of resource?
	return nil, errors.New("unknown object type " + base.Kind)
}

// For reference, the Kubernetes v1 types are in:
// https://github.com/kubernetes/client-go/blob/master/pkg/api/v1/types.go
