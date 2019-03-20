package kubernetes

import (
	"testing"
	"time"

	crdv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	crdfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	toolscache "k8s.io/client-go/tools/cache"
)

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

func TestCachedDiscovery(t *testing.T) {
	coreClient := makeFakeClient()

	myCRD := &crdv1beta1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "custom",
		},
	}
	crdClient := crdfake.NewSimpleClientset(myCRD)

	// Here's my fake API resource
	myAPI := &metav1.APIResourceList{
		GroupVersion: "foo/v1",
		APIResources: []metav1.APIResource{
			{Name: "customs", SingularName: "custom", Namespaced: true, Kind: "Custom", Verbs: getAndList},
		},
	}

	apiResources := coreClient.Fake.Resources
	coreClient.Fake.Resources = append(apiResources, myAPI)

	shutdown := make(chan struct{})
	defer close(shutdown)

	// this extra handler means we can synchronise on the add later
	// being processed
	allowAdd := make(chan interface{})

	addHandler := toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			allowAdd <- obj
		},
	}
	makeHandler := func(d discovery.CachedDiscoveryInterface) toolscache.ResourceEventHandler {
		return chainHandler{first: makeInvalidatingHandler(d), next: addHandler}
	}

	cachedDisco, store, _ := makeCachedDiscovery(coreClient.Discovery(), crdClient, shutdown, makeHandler)

	saved := getDefaultNamespace
	getDefaultNamespace = func() (string, error) { return "bar-ns", nil }
	defer func() { getDefaultNamespace = saved }()
	namespacer, err := NewNamespacer(cachedDisco)
	if err != nil {
		t.Fatal(err)
	}

	namespaced, err := namespacer.lookupNamespaced("foo/v1", "Custom")
	if err != nil {
		t.Fatal(err)
	}
	if !namespaced {
		t.Error("got false from lookupNamespaced, expecting true")
	}

	// In a cluster, we'd rely on the apiextensions server to reflect
	// changes to CRDs to changes in the API resources. Here I will be
	// more narrow, and just test that the API resources are reloaded
	// when a CRD is updated or deleted.

	// This is delicate: we can't just change the value in-place,
	// since that will update everyone's record of it, and the test
	// below will trivially succeed.
	updatedAPI := &metav1.APIResourceList{
		GroupVersion: "foo/v1",
		APIResources: []metav1.APIResource{
			{Name: "customs", SingularName: "custom", Namespaced: false /* <-- changed */, Kind: "Custom", Verbs: getAndList},
		},
	}
	coreClient.Fake.Resources = append(apiResources, updatedAPI)

	// Provoke the cached discovery client into invalidating
	_, err = crdClient.ApiextensionsV1beta1().CustomResourceDefinitions().Update(myCRD)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for the update to "go through"
	select {
	case <-allowAdd:
		break
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Add to happen")
	}

	_, exists, err := store.Get(myCRD)
	if err != nil {
		t.Error(err)
	}
	if !exists {
		t.Error("does not exist")
	}

	namespaced, err = namespacer.lookupNamespaced("foo/v1", "Custom")
	if err != nil {
		t.Fatal(err)
	}
	if namespaced {
		t.Error("got true from lookupNamespaced, expecting false (after changing it)")
	}
}
