package kubernetes

import (
	"context"
	"reflect"
	"sort"
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
	allowedNamespaces := make(map[string]struct{})
	for _, n := range namespace {
		allowedNamespaces[n] = struct{}{}
	}
	c := NewCluster(client, nil, nil, log.NewNopLogger(), allowedNamespaces, nil, []string{})

	namespaces, err := c.getAllowedAndExistingNamespaces(context.Background())
	if err != nil {
		t.Errorf("The error should be nil, not: %s", err)
	}

	sort.Strings(namespaces) // We cannot be sure of the namespace order, since they are saved in a map in cluster struct
	sort.Strings(expected)

	if reflect.DeepEqual(namespaces, expected) != true {
		t.Errorf("Unexpected namespaces: %v != %v.", namespaces, expected)
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
