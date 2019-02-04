package kubernetes

import (
	"fmt"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached"

	kresource "github.com/weaveworks/flux/cluster/kubernetes/resource"
)

type namespaceViaDiscovery struct {
	fallbackNamespace string
	disco             discovery.DiscoveryInterface
}

type namespaceDefaulter interface {
	GetDefaultNamespace() (string, error)
}

// NewNamespacer creates an implementation of Namespacer
func NewNamespacer(ns namespaceDefaulter, d discovery.DiscoveryInterface) (*namespaceViaDiscovery, error) {
	fallback, err := ns.GetDefaultNamespace()
	if err != nil {
		return nil, err
	}
	cachedDisco := cached.NewMemCacheClient(d)
	// in client-go v9, the call of Invalidate is necessary to force
	// it to query for initial data; in subsequent versions, it is a
	// no-op reset of the cache validity, so safe to call.
	cachedDisco.Invalidate()
	return &namespaceViaDiscovery{fallbackNamespace: fallback, disco: cachedDisco}, nil
}

// effectiveNamespace yields the namespace that would be used for this
// resource were it applied, taking into account the kind of the
// resource, and local configuration.
func (n *namespaceViaDiscovery) EffectiveNamespace(m kresource.KubeManifest) (string, error) {
	namespaced, err := n.lookupNamespaced(m)
	switch {
	case err != nil:
		return "", err
	case namespaced && m.GetNamespace() == "":
		return n.fallbackNamespace, nil
	case !namespaced:
		return "", nil
	}
	return m.GetNamespace(), nil
}

func (n *namespaceViaDiscovery) lookupNamespaced(m kresource.KubeManifest) (bool, error) {
	groupVersion, kind := m.GroupVersion(), m.GetKind()
	resourceList, err := n.disco.ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		return false, err
	}
	for _, resource := range resourceList.APIResources {
		if resource.Kind == kind {
			return resource.Namespaced, nil
		}
	}
	return false, fmt.Errorf("resource not found for API %s, kind %s", groupVersion, kind)
}
