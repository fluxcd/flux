package resource

import (
	"strings"

	jsonyaml "github.com/ghodss/yaml"
	"gopkg.in/yaml.v2"

	fluxerr "github.com/fluxcd/flux/pkg/errors"
	"github.com/fluxcd/flux/pkg/policy"
	"github.com/fluxcd/flux/pkg/resource"
)

const (
	PolicyPrefix       = "fluxcd.io/"
	FilterPolicyPrefix = "filter.fluxcd.io/"
	// This is the previously-used prefix for annotations; many
	// manifests in the wild will still be using it, so it's included
	// here for backward-compatibility.
	AlternatePolicyPrefix = "flux.weave.works/"
	ClusterScope          = "<cluster>"
)

// KubeManifest represents a manifest for a Kubernetes resource. For
// some Kubernetes-specific purposes we need more information that can
// be obtained from `resource.Resource`.
type KubeManifest interface {
	resource.Resource
	GroupVersion() string
	GetKind() string
	GetName() string
	GetNamespace() string
	SetNamespace(string)

	PolicyAnnotationKey(string) (string, bool)
}

// -- unmarshaling code for specific object and field types

// struct to embed in objects, to provide default implementation
type baseObject struct {
	source string
	bytes  []byte

	// these are present for unmarshalling into the struct
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Meta       struct {
		Namespace   string            `yaml:"namespace"`
		Name        string            `yaml:"name"`
		Annotations map[string]string `yaml:"annotations,omitempty"`
	} `yaml:"metadata"`
}

// GroupVersion implements KubeManifest.GroupVersion, so things with baseObject embedded are < KubeManifest
func (o baseObject) GroupVersion() string {
	return o.APIVersion
}

// GetNamespace implements KubeManifest.GetNamespace, so things embedding baseObject are < KubeManifest
func (o baseObject) GetNamespace() string {
	return o.Meta.Namespace
}

// GetKind implements KubeManifest.GetKind
func (o baseObject) GetKind() string {
	return o.Kind
}

// GetName implements KubeManifest.GetName
func (o baseObject) GetName() string {
	return o.Meta.Name
}

func (o baseObject) ResourceID() resource.ID {
	ns := o.Meta.Namespace
	if ns == "" {
		ns = ClusterScope
	}
	return resource.MakeID(ns, o.Kind, o.Meta.Name)
}

// SetNamespace implements KubeManifest.SetNamespace, so things with
// *baseObject embedded are < KubeManifest. NB pointer receiver.
func (o *baseObject) SetNamespace(ns string) {
	o.Meta.Namespace = ns
}

// It's useful for comparisons in tests to be able to remove the
// record of bytes
func (o *baseObject) debyte() {
	o.bytes = nil
}

func PoliciesFromAnnotations(annotations map[string]string) policy.Set {
	set := policy.Set{}
	for k, v := range annotations {
		var p string
		switch {
		case strings.HasPrefix(k, PolicyPrefix):
			p = strings.TrimPrefix(k, PolicyPrefix)
		case strings.HasPrefix(k, AlternatePolicyPrefix):
			p = strings.TrimPrefix(k, AlternatePolicyPrefix)
		case strings.HasPrefix(k, FilterPolicyPrefix):
			p = "tag." + strings.TrimPrefix(k, FilterPolicyPrefix)
		default:
			continue
		}

		if v == "true" {
			set = set.Add(policy.Policy(p))
		} else {
			set = set.Set(policy.Policy(p), v)
		}
	}
	return set
}

func (o baseObject) Policies() policy.Set {
	return PoliciesFromAnnotations(o.Meta.Annotations)
}

// PolicyAnnotationKey returns the key used in this resource to
// indicate a particular policy; this is to aid in supporting more
// than one way of using annotations for policy. If the policy is not
// present, returns `"", false`.
func (o baseObject) PolicyAnnotationKey(p string) (string, bool) {
	for _, prefix := range []string{PolicyPrefix, AlternatePolicyPrefix, FilterPolicyPrefix} {
		key := prefix + p
		if prefix == FilterPolicyPrefix {
			key = prefix + strings.TrimPrefix(p, "tag.")
		}
		if _, ok := o.Meta.Annotations[key]; ok {
			return key, true
		}
	}
	return "", false
}

func (o baseObject) Source() string {
	return o.source
}

func (o baseObject) Bytes() []byte {
	return o.bytes
}

func unmarshalObject(source string, bytes []byte) (KubeManifest, error) {
	var base = baseObject{source: source, bytes: bytes}
	if err := yaml.Unmarshal(bytes, &base); err != nil {
		return nil, err
	}
	r, err := unmarshalKind(base, bytes)
	if err != nil {
		return nil, makeUnmarshalObjectErr(source, err)
	}
	return r, nil
}

func unmarshalKind(base baseObject, bytes []byte) (KubeManifest, error) {
	switch {
	case base.Kind == "CronJob":
		var cj = CronJob{baseObject: base}
		if err := yaml.Unmarshal(bytes, &cj); err != nil {
			return nil, err
		}
		return &cj, nil
	case base.Kind == "DaemonSet":
		var ds = DaemonSet{baseObject: base}
		if err := yaml.Unmarshal(bytes, &ds); err != nil {
			return nil, err
		}
		return &ds, nil
	case base.Kind == "Deployment":
		var dep = Deployment{baseObject: base}
		if err := yaml.Unmarshal(bytes, &dep); err != nil {
			return nil, err
		}
		return &dep, nil
	case base.Kind == "Namespace":
		var ns = Namespace{baseObject: base}
		if err := yaml.Unmarshal(bytes, &ns); err != nil {
			return nil, err
		}
		return &ns, nil
	case base.Kind == "StatefulSet":
		var ss = StatefulSet{baseObject: base}
		if err := yaml.Unmarshal(bytes, &ss); err != nil {
			return nil, err
		}
		return &ss, nil
	case strings.HasSuffix(base.Kind, "List"):
		// All resource kinds ending with `List` are understood as
		// a list of resources. This is not bullet proof since
		// CustomResourceDefinitions can define a custom ListKind
		// (see CustomResourceDefinition.Spec.Names.ListKind) but
		// we cannot do better without involving API discovery during
		// parsing.
		var raw rawList
		if err := yaml.Unmarshal(bytes, &raw); err != nil {
			return nil, err
		}
		var list List
		unmarshalList(base, &raw, &list)
		return &list, nil
	case base.Kind == "FluxHelmRelease" || base.Kind == "HelmRelease":
		var hr = HelmRelease{baseObject: base}
		// NB: workaround for go-yaml/yaml/issues/139
		// By using github.com/ghodss/yaml to unmarshal HelmReleases.
		// This effectively results in all keys of `Value`s being strings
		// and not interface{}.
		if err := jsonyaml.Unmarshal(bytes, &hr); err != nil {
			return nil, err
		}
		return &hr, nil
	case base.Kind == "":
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

type rawList struct {
	Items []map[string]interface{}
}

func unmarshalList(base baseObject, raw *rawList, list *List) error {
	list.baseObject = base
	list.Items = make([]KubeManifest, len(raw.Items), len(raw.Items))
	for i, item := range raw.Items {
		bytes, err := yaml.Marshal(item)
		if err != nil {
			return err
		}
		res, err := unmarshalObject(base.source, bytes)
		if err != nil {
			return err
		}
		list.Items[i] = res
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
