package kubernetes

import (
	"github.com/weaveworks/flux"
	kresource "github.com/weaveworks/flux/cluster/kubernetes/resource"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/resource"
)

// namespacer assigns namespaces to manifests that need it (or "" if
// the manifest should not have a namespace.
type namespacer interface {
	// EffectiveNamespace gives the namespace that would be used were
	// the manifest to be applied. This may be "", indicating that it
	// should not have a namespace (i.e., it's a cluster-level
	// resource).
	EffectiveNamespace(kresource.KubeManifest) (string, error)
}

// Manifests is an implementation of cluster.Manifests, particular to
// Kubernetes. Aside from loading manifests from files, it does some
// "post-processsing" to make sure the view of the manifests is what
// would be applied; in particular, it fills in the namespace of
// manifests that would be given a default namespace when applied.
type Manifests struct {
	Namespacer namespacer
}

func postProcess(manifests map[string]kresource.KubeManifest, nser namespacer) (map[string]resource.Resource, error) {
	result := map[string]resource.Resource{}
	for _, km := range manifests {
		if nser != nil {
			ns, err := nser.EffectiveNamespace(km)
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
