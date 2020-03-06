package kubernetes

import (
	"io/ioutil"
	"os"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corefake "k8s.io/client-go/kubernetes/fake"

	kresource "github.com/fluxcd/flux/pkg/cluster/kubernetes/resource"
)

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
		{
			GroupVersion: "apiextensions.k8s.io/v1beta1",
			APIResources: []metav1.APIResource{
				{Name: "customresourcedefinitions", SingularName: "customresourcedefinition", Namespaced: false, Kind: "CustomResourceDefinition", Verbs: getAndList},
			},
		},
	}

	coreClient := corefake.NewSimpleClientset()
	coreClient.Fake.Resources = apiResources
	return coreClient
}

func TestNamespaceDefaulting(t *testing.T) {
	testKubeconfig := `apiVersion: v1
clusters: []
contexts:
- context:
    cluster: cluster
    namespace: namespace
    user: user
  name: context
current-context: context
kind: Config
preferences: {}
users: []
`
	err := ioutil.WriteFile("testkubeconfig", []byte(testKubeconfig), 0600)
	if err != nil {
		t.Fatal("cannot create test kubeconfig file")
	}
	defer os.Remove("testkubeconfig")

	os.Setenv("KUBECONFIG", "testkubeconfig")
	defer os.Unsetenv("KUBECONFIG")
	coreClient := makeFakeClient()

	ns, err := getKubeconfigDefaultNamespace()
	if err != nil {
		t.Fatal("cannot get default namespace")
	}
	if ns != "namespace" {
		t.Fatal("unexpected default namespace", ns)
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

	defaultNser, err := NewNamespacer(coreClient.Discovery(), "")
	if err != nil {
		t.Fatal(err)
	}
	assertEffectiveNamespace := func(nser namespaceViaDiscovery, id, expected string) {
		res, ok := manifests[id]
		if !ok {
			t.Errorf("manifest for %q not found", id)
			return
		}
		got, err := nser.EffectiveNamespace(res, nil)
		if err != nil {
			t.Errorf("error getting effective namespace for %q: %s", id, err.Error())
			return
		}
		if got != expected {
			t.Errorf("expected effective namespace of %q, got %q", expected, got)
		}
	}

	assertEffectiveNamespace(*defaultNser, "foo-ns:deployment/hasNamespace", "foo-ns")
	assertEffectiveNamespace(*defaultNser, "<cluster>:deployment/noNamespace", "namespace")
	assertEffectiveNamespace(*defaultNser, "spurious:namespace/notNamespaced", "")

	overrideNser, err := NewNamespacer(coreClient.Discovery(), "foo-override")
	if err != nil {
		t.Fatal(err)
	}

	assertEffectiveNamespace(*overrideNser, "foo-ns:deployment/hasNamespace", "foo-ns")
	assertEffectiveNamespace(*overrideNser, "<cluster>:deployment/noNamespace", "foo-override")
	assertEffectiveNamespace(*overrideNser, "spurious:namespace/notNamespaced", "")

}
