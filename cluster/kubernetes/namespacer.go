package kubernetes

import (
	"fmt"
	"sync"
	"time"

	crdv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	crd "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	discocache "k8s.io/client-go/discovery/cached"
	toolscache "k8s.io/client-go/tools/cache"

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

// This mainly exists so I can put the controller and store in here,
// so as to have access to them for testing; and, so that we can do
// our own invalidation.
type cachedDiscovery struct {
	discovery.CachedDiscoveryInterface

	store      toolscache.Store
	controller toolscache.Controller

	invalidMu sync.Mutex
	invalid   bool
}

// The older (v8.0.0) implementation of MemCacheDiscovery refreshes
// the cached values, synchronously, when Invalidate is called. Since
// we will invalidate every time something cahnges, but it only
// matters when we want to read the cached values, this method (and
// ServerResourcesForGroupVersion) saves the invalidation for when a
// read is done.
func (d *cachedDiscovery) Invalidate() {
	d.invalidMu.Lock()
	d.invalid = true
	d.invalidMu.Unlock()
}

// This happens to be the method that we call in the namespacer; so,
// this is the one where we check whether the cache has been
// invalidated. A cachedDiscovery implementation for more general use
// would do this for all methods (that weren't implemented purely in
// terms of other methods).
func (d *cachedDiscovery) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	d.invalidMu.Lock()
	invalid := d.invalid
	d.invalid = false
	d.invalidMu.Unlock()
	if invalid {
		d.CachedDiscoveryInterface.Invalidate()
	}
	return d.CachedDiscoveryInterface.ServerResourcesForGroupVersion(groupVersion)
}

type chainHandler struct {
	first toolscache.ResourceEventHandler
	next  toolscache.ResourceEventHandler
}

func (h chainHandler) OnAdd(obj interface{}) {
	h.first.OnAdd(obj)
	h.next.OnAdd(obj)
}

func (h chainHandler) OnUpdate(old, new interface{}) {
	h.first.OnUpdate(old, new)
	h.next.OnUpdate(old, new)
}

func (h chainHandler) OnDelete(old interface{}) {
	h.first.OnDelete(old)
	h.next.OnDelete(old)
}

// MakeCachedDiscovery constructs a CachedDicoveryInterface that will
// be invalidated whenever the set of CRDs change. The idea is that
// the only avenue of a change to the API resources in a running
// system is CRDs being added, updated or deleted. The prehandlers are
// there to allow us to put extra synchronisation in, for testing.
func MakeCachedDiscovery(d discovery.DiscoveryInterface, c crd.Interface, shutdown <-chan struct{}, prehandlers ...toolscache.ResourceEventHandler) *cachedDiscovery {
	cachedDisco := &cachedDiscovery{CachedDiscoveryInterface: discocache.NewMemCacheClient(d)}
	// We have an empty cache, so it's _a priori_ invalid. (Yes, that's the zero value, but better safe than sorry)
	cachedDisco.Invalidate()

	crdClient := c.ApiextensionsV1beta1().CustomResourceDefinitions()

	var handler toolscache.ResourceEventHandler = toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(_ interface{}) {
			cachedDisco.Invalidate()
		},
		UpdateFunc: func(_, _ interface{}) {
			cachedDisco.Invalidate()
		},
		DeleteFunc: func(_ interface{}) {
			cachedDisco.Invalidate()
		},
	}
	for _, h := range prehandlers {
		handler = chainHandler{first: h, next: handler}
	}

	lw := &toolscache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return crdClient.List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return crdClient.Watch(options)
		},
	}
	cachedDisco.store, cachedDisco.controller = toolscache.NewInformer(lw, &crdv1beta1.CustomResourceDefinition{}, 5*time.Minute, handler)
	go cachedDisco.controller.Run(shutdown)
	return cachedDisco
}
