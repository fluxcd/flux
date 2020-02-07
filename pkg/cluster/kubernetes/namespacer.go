package kubernetes

import (
	"github.com/argoproj/argo-cd/engine/pkg/utils/kube"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"

	kresource "github.com/fluxcd/flux/pkg/cluster/kubernetes/resource"
)

// The namespace to presume if something doesn't have one, and we
// haven't been told what to use as a fallback. This is what
// `kubectl` uses when there's no config setting the fallback
// namespace.
const defaultFallbackNamespace = "default"

type namespacerViaInfoProvider struct {
	fallbackNamespace string
	infoProvider      kube.ResourceInfoProvider
}

// If not empty `defaultNamespaceOverride` is used as the namespace when
// a resource doesn't have a namespace specified. If empty the namespace
// from the context in the KUBECONFIG is used, otherwise the "default"
// namespace is used mimicking kubectl behavior
func getFallbackNamespace(defaultNamespaceOverride string) (string, error) {
	if defaultNamespaceOverride != "" {
		return defaultNamespaceOverride, nil
	}
	kubeconfigDefaultNamespace, err := getKubeconfigDefaultNamespace()
	if err != nil {
		return "", err
	}
	if kubeconfigDefaultNamespace != "" {
		return kubeconfigDefaultNamespace, nil
	}
	return defaultFallbackNamespace, nil
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
func (n *namespacerViaInfoProvider) EffectiveNamespace(m kresource.KubeManifest, knownScopes ResourceScopes) (string, error) {
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

func (n *namespacerViaInfoProvider) lookupNamespaced(groupVersion string, kind string, knownScopes ResourceScopes) (bool, error) {
	namespaced, clusterErr := n.lookupNamespacedInCluster(groupVersion, kind)
	// FIXME: this doesn't work as it should since lookupNamespacedInCluster() doesn't return
	//        an error if the resource doesn't exist
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

func (n *namespacerViaInfoProvider) lookupNamespacedInCluster(groupVersion, kind string) (bool, error) {
	gv, err := schema.ParseGroupVersion(groupVersion)
	if err != nil {
		return false, err
	}
	vk := schema.GroupKind{
		Group: gv.Group,
		Kind:  kind,
	}
	return n.infoProvider.IsNamespaced(vk)
}
