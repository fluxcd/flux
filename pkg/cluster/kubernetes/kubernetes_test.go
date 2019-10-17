package kubernetes

import (
	"context"
	"reflect"
	"testing"

	"github.com/go-kit/kit/log"
	apiv1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekubernetes "k8s.io/client-go/kubernetes/fake"
)

func newNamespace(name string) *apiv1.Namespace {
	return &apiv1.Namespace{
		ObjectMeta: meta_v1.ObjectMeta{
			Name: name,
		},
		TypeMeta: meta_v1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
	}
}

func testGetAllowedNamespaces(t *testing.T, namespace []string, expected []string) {
	clientset := fakekubernetes.NewSimpleClientset(newNamespace("default"),
		newNamespace("kube-system"))
	client := ExtendedClient{coreClient: clientset}
	c := NewCluster(client, nil, nil, log.NewNopLogger(), namespace, []string{})

	namespaces, err := c.getAllowedAndExistingNamespaces(context.Background())
	if err != nil {
		t.Errorf("The error should be nil, not: %s", err)
	}

	result := []string{}
	for _, namespace := range namespaces {
		result = append(result, namespace)
	}

	if reflect.DeepEqual(result, expected) != true {
		t.Errorf("Unexpected namespaces: %v != %v.", result, expected)
	}
}

func TestGetAllowedNamespacesDefault(t *testing.T) {
	testGetAllowedNamespaces(t, []string{}, []string{""}) // this will be empty string which means all namespaces
}

func TestGetAllowedNamespacesNamespacesIsNil(t *testing.T) {
	testGetAllowedNamespaces(t, nil, []string{""}) // this will be empty string which means all namespaces
}

func TestGetAllowedNamespacesNamespacesSet(t *testing.T) {
	testGetAllowedNamespaces(t, []string{"default"}, []string{"default"})
}

func TestGetAllowedNamespacesNamespacesSetDoesNotExist(t *testing.T) {
	testGetAllowedNamespaces(t, []string{"hello"}, []string{})
}

func TestGetAllowedNamespacesNamespacesMultiple(t *testing.T) {
	testGetAllowedNamespaces(t, []string{"default", "hello", "kube-system"}, []string{"default", "kube-system"})
}
