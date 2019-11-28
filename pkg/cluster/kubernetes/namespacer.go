package kubernetes

import (
	"fmt"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/clientcmd"

	kresource "github.com/fluxcd/flux/pkg/cluster/kubernetes/resource"
)

// The namespace to presume if something doesn't have one, and we
// haven't been told what to use as a fallback. This is what
// `kubectl` uses when there's no config setting the fallback
// namespace.
const defaultFallbackNamespace = "default"

type namespaceViaDiscovery struct {
	fallbackNamespace string
	disco             discovery.DiscoveryInterface
}

// NewNamespacer creates an implementation of Namespacer
// If not empty `defaultNamespaceOverride` is used as the namespace when
// a resource doesn't have a namespace specified. If empty the namespace
// from the context in the KUBECONFIG is used, otherwise the "default"
// namespace is used mimicking kubectl behavior
func NewNamespacer(d discovery.DiscoveryInterface, defaultNamespaceOverride string) (*namespaceViaDiscovery, error) {
	if defaultNamespaceOverride != "" {
		return &namespaceViaDiscovery{fallbackNamespace: defaultNamespaceOverride, disco: d}, nil
	}
	kubeconfigDefaultNamespace, err := getKubeconfigDefaultNamespace()
	if err != nil {
		return nil, err
	}
	if kubeconfigDefaultNamespace != "" {
		return &namespaceViaDiscovery{fallbackNamespace: kubeconfigDefaultNamespace, disco: d}, nil
	}
	return &namespaceViaDiscovery{fallbackNamespace: defaultFallbackNamespace, disco: d}, nil
}

// getKubeconfigDefaultNamespace returns the namespace specified
// for the current config in KUBECONFIG
// A variable is used for mocking in tests.
var getKubeconfigDefaultNamespace = func() (string, error) {
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	).RawConfig()
	if err != nil {
		return "", err
	}

	cc := config.CurrentContext
	if c, ok := config.Contexts[cc]; ok && c.Namespace != "" {
		return c.Namespace, nil
	}

	return "", nil
}

// effectiveNamespace yields the namespace that would be used for this
// resource were it applied, taking into account the kind of the
// resource, and local configuration.
func (n *namespaceViaDiscovery) EffectiveNamespace(m kresource.KubeManifest, knownScopes ResourceScopes) (string, error) {
	namespaced, err := n.lookupNamespaced(m.GroupVersion(), m.GetKind(), knownScopes)
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

func (n *namespaceViaDiscovery) lookupNamespaced(groupVersion string, kind string, knownScopes ResourceScopes) (bool, error) {
	namespaced, clusterErr := n.lookupNamespacedInCluster(groupVersion, kind)
	if clusterErr == nil || knownScopes == nil {
		return namespaced, nil
	}
	// Not found in the cluster, let's try the known scopes
	gv, err := schema.ParseGroupVersion(groupVersion)
	if err != nil {
		return false, clusterErr
	}
	scope, found := knownScopes[gv.WithKind(kind)]
	if !found {
		return false, clusterErr
	}
	return scope == v1beta1.NamespaceScoped, nil
}

func (n *namespaceViaDiscovery) lookupNamespacedInCluster(groupVersion, kind string) (bool, error) {
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
