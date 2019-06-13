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
	"k8s.io/client-go/discovery/cached/memory"
	toolscache "k8s.io/client-go/tools/cache"
)

// This exists so that we can do our own invalidation.
type cachedDiscovery struct {
	discovery.CachedDiscoveryInterface

	invalidMu sync.Mutex
	invalid   bool
}

// The k8s.io/client-go v8.0.0 implementation of MemCacheDiscovery
// refreshes the cached values, synchronously, when Invalidate() is
// called. Since we want to invalidate every time a CRD changes, but
// only refresh values when we need to read the cached values, this
// method defers the invalidation until a read is done.
func (d *cachedDiscovery) Invalidate() {
	d.invalidMu.Lock()
	d.invalid = true
	d.invalidMu.Unlock()
}

// ServerResourcesForGroupVersion is the method used by the
// namespacer; so, this is the one where we check whether the cache
// has been invalidated. A cachedDiscovery implementation for more
// general use would do this for all methods (that weren't implemented
// purely in terms of other methods).
func (d *cachedDiscovery) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	d.invalidMu.Lock()
	invalid := d.invalid
	d.invalid = false
	d.invalidMu.Unlock()
	if invalid {
		d.CachedDiscoveryInterface.Invalidate()
	}
	result, err := d.CachedDiscoveryInterface.ServerResourcesForGroupVersion(groupVersion)
	if err == memory.ErrCacheNotFound {
		// improve the error returned from memcacheclient
		err = fmt.Errorf("server resources for %s not found in cache; cache refreshes every 5 minutes", groupVersion)
	}
	return result, err
}

// MakeCachedDiscovery constructs a CachedDicoveryInterface that will
// be invalidated whenever the set of CRDs change. The idea is that
// the only avenue of a change to the API resources in a running
// system is CRDs being added, updated or deleted.
func MakeCachedDiscovery(d discovery.DiscoveryInterface, c crd.Interface, shutdown <-chan struct{}) discovery.CachedDiscoveryInterface {
	cachedDisco := &cachedDiscovery{CachedDiscoveryInterface: memory.NewMemCacheClient(d)}
	// We have an empty cache, so it's _a priori_ invalid. (Yes, that's the zero value, but better safe than sorry)
	cachedDisco.Invalidate()

	crdClient := c.ApiextensionsV1beta1().CustomResourceDefinitions()
	lw := &toolscache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return crdClient.List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return crdClient.Watch(options)
		},
	}
	handle := toolscache.ResourceEventHandlerFuncs{
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
	_, controller := toolscache.NewInformer(lw, &crdv1beta1.CustomResourceDefinition{}, 0, handle)
	go cachedDisco.invalidatePeriodically(shutdown)
	go controller.Run(shutdown)
	return cachedDisco
}

func (d *cachedDiscovery) invalidatePeriodically(shutdown <-chan struct{}) {
	// Make the first period shorter since we may be bootstrapping a
	// newly-created cluster and the resource definitions may not be
	// complete/stable yet.
	initialPeriodDuration := 1 * time.Minute
	subsequentPeriodsDuration := 5 * initialPeriodDuration
	timer := time.NewTimer(initialPeriodDuration)
	for {
		select {
		case <-timer.C:
			timer.Reset(subsequentPeriodsDuration)
			d.Invalidate()
		case <-shutdown:
			timer.Stop()
			return
		}
	}
}
