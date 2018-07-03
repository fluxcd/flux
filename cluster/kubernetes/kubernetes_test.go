package kubernetes

import (
	"testing"

	"github.com/go-kit/kit/log"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

func kubeTestSetup(t *testing.T, nsWhitelist []string) (*Cluster, apiv1.NamespaceList) {
	kube := &Cluster{
		applier: nil,
		logger:  log.NewNopLogger(),
		nsWhitelist: nsWhitelist,
	}

	// Create a new API object from scratch each time
	namespaces := []apiv1.Namespace{}
	for _, nsName := range testNamespaces {
		namespaces = append(namespaces, apiv1.Namespace{
			TypeMeta: v1.TypeMeta{
				Kind: "Namespace",
			},
			ObjectMeta: v1.ObjectMeta{
				Name: nsName,
			},
		})
	}
	namespaceList :=  apiv1.NamespaceList{Items: namespaces}
	return kube, namespaceList
}

func TestFilterNamespacesEmptyFilter(t *testing.T) {
	kube, namespaces := kubeTestSetup(t, []string{})

	kube.filterNamespaces(&namespaces)
	for i, _ := range testNamespaces {
		filteredName := namespaces.Items[i].Name
		unfilteredName := testNamespaces[i]
		if filteredName != unfilteredName {
			t.Errorf("expected namespace '%s' but found '%s'", unfilteredName, filteredName)
		}
	}
}

func TestFilterNamespacesOneNamespace(t *testing.T) {
	kube, namespaces := kubeTestSetup(t, []string{"namespace2"})

	kube.filterNamespaces(&namespaces)

	if len(namespaces.Items) != 1 {
		t.Errorf("expected 1 namespace but got %d - %#v", len(namespaces.Items), namespaces.Items)
	}
	if namespaces.Items[0].Name != "namespace2" {
		t.Errorf("expected namespace 'namespace2' but was '%s'", namespaces.Items[0].Name)
	}
}

func TestFilterNamespacesTwoNamespaces(t *testing.T) {
	kube, namespaces := kubeTestSetup(t, []string{"namespace2", "namespace1"})

	kube.filterNamespaces(&namespaces)

	if len(namespaces.Items) != 2 {
		t.Errorf("expected 2 namespaces but got %d - %#v", len(namespaces.Items), namespaces.Items)
	}
	for i, expected := range []string{"namespace1", "namespace2"} {
		if namespaces.Items[i].Name != expected {
			t.Errorf("expected list element %d to be '%s' but was '%s'", i, expected, namespaces.Items[0].Name)
		}
	}
}

var testNamespaces = [...]string{
	"namespace1",
	"namespace2",
	"namespace3",
}
