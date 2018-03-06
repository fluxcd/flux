package resource

import (
	"strings"

	yaml "gopkg.in/yaml.v2"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	fluxerr "github.com/weaveworks/flux/errors"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
)

const (
	PolicyPrefix = "flux.weave.works/"
)

// -- unmarshaling code for specific object and field types

// struct to embed in objects, to provide default implementation
type BaseObject struct {
	source     string
	bytes      []byte
	APIVersion string   `yaml:"apiVersion,omitempty"`
	Metadata   Metadata `yaml:"metadata,omitempty"`
	Kind       string   `yaml:"kind"`
	Spec       struct {
		Template struct {
			Metadata Metadata
			Spec     struct {
				Containers []Container `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
		JobTemplate struct {
			Spec struct {
				Template struct {
					Spec struct {
						Containers []Container `yaml:"containers"`
					} `yaml:"spec"`
				} `yaml:"template"`
			} `yaml:"spec"`
		} `yaml:"jobTemplate,omitempty"`
	} `yaml:"spec,omitempty"`
}

type Metadata struct {
	Name        string            `yaml:"name,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
	Namespace   string            `yaml:"namespace,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
}

type Container struct {
	Name  string `yaml:"name"`
	Image string `yaml:"image"`
}

func (m Metadata) AnnotationsOrNil() map[string]string {
	if m.Annotations == nil {
		return map[string]string{}
	}
	return m.Annotations
}

func (o BaseObject) ResourceID() flux.ResourceID {
	ns := o.Metadata.Namespace
	if ns == "" {
		ns = "default"
	}
	return flux.MakeResourceID(ns, o.Kind, o.Metadata.Name)
}

// It's useful for comparisons in tests to be able to remove the
// record of bytes
func (o *BaseObject) debyte() {
	o.bytes = nil
}

func (o BaseObject) Policy() policy.Set {
	set := policy.Set{}
	for k, v := range o.Metadata.Annotations {
		if strings.HasPrefix(k, PolicyPrefix) {
			p := strings.TrimPrefix(k, PolicyPrefix)
			if v == "true" {
				set = set.Add(policy.Policy(p))
			} else {
				set = set.Set(policy.Policy(p), v)
			}
		}
	}
	return set
}

func (o BaseObject) Source() string {
	return o.source
}

func (o BaseObject) Bytes() []byte {
	return o.bytes
}

func unmarshalObject(source string, bytes []byte) (*BaseObject, error) {
	var base = BaseObject{source: source, bytes: bytes}
	if err := yaml.Unmarshal(bytes, &base); err != nil {
		return nil, err
	}

	return &base, nil
}

func unmarshalKind(base BaseObject, bytes []byte) (resource.Resource, error) {
	switch base.Kind {
	case "CronJob":
		var cj = CronJob{BaseObject: base}
		if err := yaml.Unmarshal(bytes, &cj); err != nil {
			return nil, err
		}
		return &cj, nil
	case "DaemonSet":
		var ds = DaemonSet{BaseObject: base}
		if err := yaml.Unmarshal(bytes, &ds); err != nil {
			return nil, err
		}
		return &ds, nil
	case "Deployment":
		var dep = Deployment{BaseObject: base}
		if err := yaml.Unmarshal(bytes, &dep); err != nil {
			return nil, err
		}
		return &dep, nil
	case "Namespace":
		var ns = Namespace{BaseObject: base}
		if err := yaml.Unmarshal(bytes, &ns); err != nil {
			return nil, err
		}
		return &ns, nil
	case "StatefulSet":
		var ss = StatefulSet{BaseObject: base}
		if err := yaml.Unmarshal(bytes, &ss); err != nil {
			return nil, err
		}
		return &ss, nil
	case "":
		// If there is an empty resource (due to eg an introduced comment),
		// we are returning nil for the resource and nil for an error
		// (as not really an error). We are not, at least at the moment,
		// reporting an error for invalid non-resource yamls on the
		// assumption it is unlikely to happen.
		return nil, nil
	// The remainder are things we have to care about, but not
	// treat specially
	default:
		return &base, nil
	}
}

func unmarshalList(source string, base *BaseObject, collection map[string]resource.Resource) error {
	// Keep each List item in this generic type.
	// We need to preserve this to be able to pass through arbitrary objects, such as Services, for Sync jobs.
	list := map[string]interface{}{}
	err := yaml.Unmarshal(base.Bytes(), &list)

	if err != nil {
		return err
	}

	for _, item := range list["items"].([]interface{}) {
		// Re-marshal this snippet. We need this item represented in byte form.
		// This will eventually be used to generate an update here:
		// https://github.com/weaveworks/flux/blob/master/cluster/sync.go#L9
		itemBytes, err := yaml.Marshal(item)

		if err != nil {
			return makeUnmarshalObjectErr(source, err)
		}

		i := BaseObject{}
		err = yaml.Unmarshal(itemBytes, &i)

		if err != nil {
			return makeUnmarshalObjectErr(source, err)
		}

		i.source = source
		i.bytes = itemBytes
		r, err := unmarshalKind(i, itemBytes)

		if err != nil {
			return makeUnmarshalObjectErr(source, err)
		}

		if r == nil {
			continue
		}

		// Catch if a resource is defined already, either via a List or a file
		if resourceAlreadyExists(collection, r) {
			return cluster.ErrMultipleResourceDefinitionsFoundForService
		}

		collection[r.ResourceID().String()] = r
	}

	return nil
}

func makeUnmarshalObjectErr(source string, err error) *fluxerr.Error {
	return &fluxerr.Error{
		Type: fluxerr.User,
		Err:  err,
		Help: `Could not parse "` + source + `".

This likely means it is malformed YAML.
`,
	}
}

// For reference, the Kubernetes v1 types are in:
// https://github.com/kubernetes/client-go/blob/master/pkg/api/v1/types.go
