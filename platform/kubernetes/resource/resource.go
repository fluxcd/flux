package resource

import (
	"errors"

	yaml "gopkg.in/yaml.v2"

	"github.com/weaveworks/flux/diff"
)

// -- unmarshaling code and diffing code for specific object and field
// types

// struct to embed in objects, to provide default implementation
type baseObject struct {
	source string
	Kind   string `yaml:"kind"`
	Meta   struct {
		Namespace string `yaml:"namespace"`
		Name      string `yaml:"name"`
	} `yaml:"metadata"`
}

func (o baseObject) ID() diff.ObjectID {
	ns := o.Meta.Namespace
	if ns == "" {
		ns = "default"
	}
	return diff.ObjectID{
		Kind:      o.Kind,
		Namespace: ns,
		Name:      o.Meta.Name,
	}
}

func (o baseObject) Source() string {
	return o.source
}

func unmarshalObject(source string, bytes []byte) (diff.Object, error) {
	var base = baseObject{source: source}
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
	case "Secret":
		var secret = Secret{baseObject: base}
		if err := yaml.Unmarshal(bytes, &secret); err != nil {
			return nil, err
		}
		return &secret, nil
	case "ConfigMap":
		var config = ConfigMap{baseObject: base}
		if err := yaml.Unmarshal(bytes, &config); err != nil {
			return nil, err
		}
		return &config, nil
	case "Namespace":
		var ns = Namespace{baseObject: base}
		if err := yaml.Unmarshal(bytes, &ns); err != nil {
			return nil, err
		}
		return &ns, nil
	case "DaemonSet":
		var ds = DaemonSet{baseObject: base}
		if err := yaml.Unmarshal(bytes, &ds); err != nil {
			return nil, err
		}
		return &ds, nil
	case "Node":
		var n = Node{baseObject: base}
		if err := yaml.Unmarshal(bytes, &n); err != nil {
			return nil, err
		}
		return &n, nil
	}

	return nil, errors.New("unknown object type " + base.Kind)
}

// For reference, the Kubernetes v1 types are in:
// https://github.com/kubernetes/client-go/blob/master/pkg/api/v1/types.go
