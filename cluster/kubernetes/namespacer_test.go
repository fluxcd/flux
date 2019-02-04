package kubernetes

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corefake "k8s.io/client-go/kubernetes/fake"

	kresource "github.com/weaveworks/flux/cluster/kubernetes/resource"
)

type namespaceDefaulterFake string

func (ns namespaceDefaulterFake) GetDefaultNamespace() (string, error) {
	return string(ns), nil
}

func TestNamespaceDefaulting(t *testing.T) {

	getAndList := metav1.Verbs([]string{"get", "list"})
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
	disco := coreClient.Discovery()
	nser, err := NewNamespacer(namespaceDefaulterFake("fallback-ns"), disco)
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
