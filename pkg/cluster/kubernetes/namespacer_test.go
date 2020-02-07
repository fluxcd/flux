package kubernetes

import (
	"errors"
	"io/ioutil"
	"os"
	"testing"

	"github.com/argoproj/argo-cd/engine/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/runtime/schema"

	kresource "github.com/fluxcd/flux/pkg/cluster/kubernetes/resource"
)

type mockResourceInfoProvider struct {
	isNamespaced map[schema.GroupKind]bool
}

func (m *mockResourceInfoProvider) IsNamespaced(gk schema.GroupKind) (bool, error) {
	if namespaced, ok := m.isNamespaced[gk]; ok {
		return namespaced, nil
	}
	return false, errors.New("not found")
}

func newNamespacer(defaultNamespace string, scoper kube.ResourceInfoProvider) (*namespacerViaInfoProvider, error) {
	fallbackNamespace, err := getFallbackNamespace(defaultNamespace)
	if err != nil {
		return nil, err
	}
	return &namespacerViaInfoProvider{infoProvider: scoper, fallbackNamespace: fallbackNamespace}, nil
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

	scoper := &mockResourceInfoProvider{map[schema.GroupKind]bool{{"apps", "Deployment"}: true}}
	defaultNser, err := newNamespacer("", scoper)
	if err != nil {
		t.Fatal(err)
	}
	assertEffectiveNamespace := func(nser namespacerViaInfoProvider, id, expected string) {
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

	overrideNser, err := newNamespacer("foo-override", scoper)
	if err != nil {
		t.Fatal(err)
	}

	assertEffectiveNamespace(*overrideNser, "foo-ns:deployment/hasNamespace", "foo-ns")
	assertEffectiveNamespace(*overrideNser, "<cluster>:deployment/noNamespace", "foo-override")
	assertEffectiveNamespace(*overrideNser, "spurious:namespace/notNamespaced", "")

}
