package kubernetes

import (
	"gopkg.in/yaml.v2"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/weaveworks/flux"
	kresource "github.com/weaveworks/flux/cluster/kubernetes/resource"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/resource"
)

// ResourceScopes maps resource definitions (GroupVersionKind) to whether they are namespaced or not
type ResourceScopes map[schema.GroupVersionKind]v1beta1.ResourceScope

// namespacer assigns namespaces to manifests that need it (or "" if
// the manifest should not have a namespace.
type namespacer interface {
	// EffectiveNamespace gives the namespace that would be used were
	// the manifest to be applied. This may be "", indicating that it
	// should not have a namespace (i.e., it's a cluster-level
	// resource).
	EffectiveNamespace(manifest kresource.KubeManifest, knownScopes ResourceScopes) (string, error)
}

// Manifests is an implementation of cluster.Manifests, particular to
// Kubernetes. Aside from loading manifests from files, it does some
// "post-processsing" to make sure the view of the manifests is what
// would be applied; in particular, it fills in the namespace of
// manifests that would be given a default namespace when applied.
type Manifests struct {
	Namespacer namespacer
}

func getCRDScopes(manifests map[string]kresource.KubeManifest) ResourceScopes {
	result := ResourceScopes{}
	for _, km := range manifests {
		if km.GetKind() == "CustomResourceDefinition" {
			var crd v1beta1.CustomResourceDefinition
			if err := yaml.Unmarshal(km.Bytes(), &crd); err != nil {
				// The CRD can't be parsed, so we (intentionally) ignore it and
				// just hope for EffectiveNamespace() to find its scope in the cluster if needed.
				continue
			}
			crdVersions := crd.Spec.Versions
			if len(crdVersions) == 0 {
				crdVersions = []v1beta1.CustomResourceDefinitionVersion{{Name: crd.Spec.Version}}
			}
			for _, crdVersion := range crdVersions {
				gvk := schema.GroupVersionKind{
					Group:   crd.Spec.Group,
					Version: crdVersion.Name,
					Kind:    crd.Spec.Names.Kind,
				}
				result[gvk] = crd.Spec.Scope
			}
		}
	}
	return result
}

func postProcess(manifests map[string]kresource.KubeManifest, nser namespacer) (map[string]resource.Resource, error) {
	knownScopes := getCRDScopes(manifests)
	result := map[string]resource.Resource{}
	for _, km := range manifests {
		if nser != nil {
			ns, err := nser.EffectiveNamespace(km, knownScopes)
			if err != nil {
				return nil, err
			}
			km.SetNamespace(ns)
		}
		result[km.ResourceID().String()] = km
	}
	return result, nil
}

func (c *Manifests) LoadManifests(base string, paths []string) (map[string]resource.Resource, error) {
	manifests, err := kresource.Load(base, paths)
	if err != nil {
		return nil, err
	}
	return postProcess(manifests, c.Namespacer)
}

func (c *Manifests) UpdateImage(def []byte, id flux.ResourceID, container string, image image.Ref) ([]byte, error) {
	return updateWorkload(def, id, container, image)
}

// UpdatePolicies and ServicesWithPolicies in policies.go
