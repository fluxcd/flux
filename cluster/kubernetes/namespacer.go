package kubernetes

import (
	"fmt"

	"k8s.io/client-go/discovery"

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
	return &namespaceViaDiscovery{fallbackNamespace: fallback, disco: d}, nil
}

// effectiveNamespace yields the namespace that would be used for this
// resource were it applied, taking into account the kind of the
// resource, and local configuration.
func (n *namespaceViaDiscovery) EffectiveNamespace(m kresource.KubeManifest) (string, error) {
	namespaced, err := n.lookupNamespaced(m.GroupVersion(), m.GetKind())
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

func (n *namespaceViaDiscovery) lookupNamespaced(groupVersion, kind string) (bool, error) {
	resourceList, err := n.disco.ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		return false, fmt.Errorf("error looking up API resources for %s.%s: %s", kind, groupVersion, err.Error())
	}
	for _, resource := range resourceList.APIResources {
		if resource.Kind == kind {
			return resource.Namespaced, nil
		}
	}
	return false, fmt.Errorf("resource not found for API %s, kind %s", groupVersion, kind)
}
