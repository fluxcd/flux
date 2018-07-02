package kubernetes

import (
	apiv1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekubernetes "k8s.io/client-go/kubernetes/fake"
	"testing"
	"reflect"
)

func newNamespace(name string) *apiv1.Namespace {
	return &apiv1.Namespace{
		ObjectMeta: meta_v1.ObjectMeta{
			Name: name,
		},
		TypeMeta: meta_v1.TypeMeta{
			APIVersion: "v1",
			Kind: "Namespace",
		},
	}
}

func testGetNamespaces(t *testing.T, namespace []string, expected []string) {
	clientset := fakekubernetes.NewSimpleClientset(newNamespace("default"),
		newNamespace("kube-system"))

	c := NewCluster(clientset, nil, nil, nil, nil, namespace)

	namespaces, err := c.getNamespaces()
	if err != nil {
		t.Errorf("The error should be nil, not: %s", err)
	}

	result := []string{}
	for _, namespace := range namespaces {
		result = append(result, namespace.ObjectMeta.Name)
	}

	if reflect.DeepEqual(result, expected) != true {
		t.Errorf("Unexpected namespaces: %v != %v.", result, expected)
	}
}

func TestGetNamespacesDefault(t *testing.T) {
	testGetNamespaces(t, []string{}, []string{"default","kube-system",})
}

func TestGetNamespacesNamespacesIsNil(t *testing.T) {
	testGetNamespaces(t, nil, []string{"default","kube-system",})
}

func TestGetNamespacesNamespacesSet(t *testing.T) {
	testGetNamespaces(t, []string{"default"}, []string{"default",})
}

func TestGetNamespacesNamespacesSetDoesNotExist(t *testing.T) {
	testGetNamespaces(t, []string{"hello"}, []string{})
}

func TestGetNamespacesNamespacesMultiple(t *testing.T) {
	testGetNamespaces(t, []string{"default","hello","kube-system"}, []string{"default","kube-system"})
}
