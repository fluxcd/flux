package kubernetes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	crdv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	crdfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

	cachedDisco := MakeCachedDiscovery(coreClient.Discovery(), crdClient, shutdown)

	saved := getKubeconfigDefaultNamespace
	getKubeconfigDefaultNamespace = func() (string, error) { return "bar-ns", nil }
	defer func() { getKubeconfigDefaultNamespace = saved }()
	namespacer, err := NewNamespacer(cachedDisco, "")
	if err != nil {
		t.Fatal(err)
	}

	namespaced, err := namespacer.lookupNamespaced("foo/v1", "Custom", nil)
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
	c := time.After(time.Second)
loop:
	for {
		select {
		default:
			namespaced, err = namespacer.lookupNamespaced("foo/v1", "Custom", nil)
			assert.NoError(t, err)
			if !namespaced {
				break loop
			}
			time.Sleep(10 * time.Millisecond)
		case <-c:
			t.Fatal("timed out waiting for Update to happen")
		}
	}
}
