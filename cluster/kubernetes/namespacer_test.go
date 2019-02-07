package kubernetes

import (
	"testing"
	"time"

	crdv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	crdfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corefake "k8s.io/client-go/kubernetes/fake"
	toolscache "k8s.io/client-go/tools/cache"

	kresource "github.com/weaveworks/flux/cluster/kubernetes/resource"
)

type namespaceDefaulterFake string

func (ns namespaceDefaulterFake) GetDefaultNamespace() (string, error) {
	return string(ns), nil
}

var getAndList = metav1.Verbs([]string{"get", "list"})

func makeFakeClient() *corefake.Clientset {
	apiResources := []*metav1.APIResourceList{
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", SingularName: "deployment", Namespaced: true, Kind: "Deployment", Verbs: getAndList},
			},
		},
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "namespaces", SingularName: "namespace", Namespaced: false, Kind: "Namespace", Verbs: getAndList},
			},
		},
	}

	coreClient := corefake.NewSimpleClientset()
	coreClient.Fake.Resources = apiResources
	return coreClient
}

func TestNamespaceDefaulting(t *testing.T) {
	coreClient := makeFakeClient()
	nser, err := NewNamespacer(namespaceDefaulterFake("fallback-ns"), coreClient.Discovery())
	if err != nil {
		t.Fatal(err)
	}

	const defs = `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hasNamespace
  namespace: foo-ns
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: noNamespace
---
apiVersion: v1
kind: Namespace
metadata:
  name: notNamespaced
  namespace: spurious
`

	manifests, err := kresource.ParseMultidoc([]byte(defs), "<string>")
	if err != nil {
		t.Fatal(err)
	}

	assertEffectiveNamespace := func(id, expected string) {
		res, ok := manifests[id]
		if !ok {
			t.Errorf("manifest for %q not found", id)
			return
		}
		got, err := nser.EffectiveNamespace(res)
		if err != nil {
			t.Errorf("error getting effective namespace for %q: %s", id, err.Error())
			return
		}
		if got != expected {
			t.Errorf("expected effective namespace of %q, got %q", expected, got)
		}
	}

	assertEffectiveNamespace("foo-ns:deployment/hasNamespace", "foo-ns")
	assertEffectiveNamespace("<cluster>:deployment/noNamespace", "fallback-ns")
	assertEffectiveNamespace("spurious:namespace/notNamespaced", "")
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
	handler := toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			allowAdd <- obj
		},
	}

	cachedDisco := MakeCachedDiscovery(coreClient.Discovery(), crdClient, shutdown, handler)

	namespacer, err := NewNamespacer(namespaceDefaulterFake("bar-ns"), cachedDisco)
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

	_, exists, err := cachedDisco.store.Get(myCRD)
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
