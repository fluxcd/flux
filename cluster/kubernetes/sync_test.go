package kubernetes

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/go-kit/kit/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	//	"k8s.io/apimachinery/pkg/runtime/serializer"
	//	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	//	dynamicfake "k8s.io/client-go/dynamic/fake"
	//	k8sclient "k8s.io/client-go/kubernetes"
	corefake "k8s.io/client-go/kubernetes/fake"
	k8s_testing "k8s.io/client-go/testing"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	fluxfake "github.com/weaveworks/flux/integrations/client/clientset/versioned/fake"
	"github.com/weaveworks/flux/sync"
)

func fakeClients() extendedClient {
	scheme := runtime.NewScheme()

	// Set this to `true` to output a trace of the API actions called
	// while running the tests
	const debug = false

	getAndList := metav1.Verbs([]string{"get", "list"})
	// Adding these means the fake dynamic client will find them, and
	// be able to enumerate (list and get) the resources that we care
	// about
	apiResources := []*metav1.APIResourceList{
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", SingularName: "deployment", Namespaced: true, Kind: "Deployment", Verbs: getAndList},
			},
		},
	}

	coreClient := corefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "foobar"}})
	fluxClient := fluxfake.NewSimpleClientset()
	dynamicClient := NewSimpleDynamicClient(scheme) // NB from this package, rather than the official one, since we needed a patched version

	// Assigned here, since this is _also_ used by the (fake)
	// discovery client therein, and ultimately by
	// getResourcesInStack since that uses the core clientset to
	// enumerate the namespaces.
	coreClient.Fake.Resources = apiResources

	if debug {
		for _, fake := range []*k8s_testing.Fake{&coreClient.Fake, &fluxClient.Fake, &dynamicClient.Fake} {
			fake.PrependReactor("*", "*", func(action k8s_testing.Action) (bool, runtime.Object, error) {
				gvr := action.GetResource()
				println("[DEBUG] action: ", action.GetVerb(), gvr.Group, gvr.Version, gvr.Resource)
				return false, nil, nil
			})
		}
	}

	return extendedClient{
		coreClient:     coreClient,
		fluxHelmClient: fluxClient,
		dynamicClient:  dynamicClient,
	}
}

// fakeApplier is an Applier that just forwards changeset operations
// to a dynamic client. It doesn't try to properly patch resources
// when that might be expected; it just overwrites them. But this is
// enough for checking whether sync operations succeeded and had the
// correct effect, which is either to "upsert", or delete, resources.
type fakeApplier struct {
	client     dynamic.Interface
	commandRun bool
}

func (a fakeApplier) apply(_ log.Logger, cs changeSet, errored map[flux.ResourceID]error) cluster.SyncError {
	var errs []cluster.ResourceError

	operate := func(obj applyObject, cmd string) {
		a.commandRun = true
		var unstruct map[string]interface{}
		if err := yaml.Unmarshal(obj.Payload, &unstruct); err != nil {
			errs = append(errs, cluster.ResourceError{obj.ResourceID, obj.Source, err})
			return
		}
		res := &unstructured.Unstructured{Object: unstruct}

		// This is a special case trapdoor, for testing failure to
		// apply a resource.
		if errStr := res.GetAnnotations()["error"]; errStr != "" {
			errs = append(errs, cluster.ResourceError{obj.ResourceID, obj.Source, fmt.Errorf(errStr)})
			return
		}

		gvk := res.GetObjectKind().GroupVersionKind()
		gvr := schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: strings.ToLower(gvk.Kind) + "s"}
		c := a.client.Resource(gvr)
		var dc dynamic.ResourceInterface = c
		if ns := res.GetNamespace(); ns != "" {
			dc = c.Namespace(ns)
		}
		name := res.GetName()

		if cmd == "apply" {
			_, err := dc.Get(name, metav1.GetOptions{})
			switch {
			case errors.IsNotFound(err):
				_, err = dc.Create(res) //, &metav1.CreateOptions{})
			case err == nil:
				_, err = dc.Update(res) //, &metav1.UpdateOptions{})
			}
			if err != nil {
				errs = append(errs, cluster.ResourceError{obj.ResourceID, obj.Source, err})
				return
			}
		} else if cmd == "delete" {
			if err := dc.Delete(name, &metav1.DeleteOptions{}); err != nil {
				errs = append(errs, cluster.ResourceError{obj.ResourceID, obj.Source, err})
				return
			}
		} else {
			panic("unknown action: " + cmd)
		}
	}

	for _, obj := range cs.objs["delete"] {
		operate(obj, "delete")
	}
	for _, obj := range cs.objs["apply"] {
		operate(obj, "apply")
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// ---

func setup(t *testing.T) (*Cluster, *fakeApplier) {
	clients := fakeClients()
	applier := &fakeApplier{client: clients.dynamicClient}
	kube := &Cluster{
		applier: applier,
		client:  clients,
		logger:  log.NewNopLogger(),
	}
	return kube, applier
}

func TestSyncNop(t *testing.T) {
	kube, mock := setup(t)
	if err := kube.Sync(cluster.SyncDef{}); err != nil {
		t.Errorf("%#v", err)
	}
	if mock.commandRun {
		t.Error("expected no commands run")
	}
}

func TestSync(t *testing.T) {
	const defs1 = `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dep1
  namespace: foobar
`

	const defs2 = `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dep2
  namespace: foobar
`

	const defs3 = `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dep3
  namespace: other
`

	test := func(t *testing.T, kube *Cluster, defs, expectedAfterSync string, expectErrors bool) {
		manifests := &Manifests{}
		resources, err := manifests.ParseManifests([]byte(defs))
		if err != nil {
			t.Fatal(err)
		}

		err = sync.Sync(log.NewNopLogger(), manifests, resources, kube)
		if !expectErrors && err != nil {
			t.Error(err)
		}
		resources0, err := manifests.ParseManifests([]byte(expectedAfterSync))
		if err != nil {
			panic(err)
		}

		// Now check that the resources were created
		resources1, err := kube.getResourcesInStack()
		if err != nil {
			t.Fatal(err)
		}

		for id := range resources1 {
			if _, ok := resources0[id]; !ok {
				t.Errorf("resource present after sync but not in resources applied: %q", id)
			}
		}
		for id := range resources0 {
			if _, ok := resources1[id]; !ok {
				t.Errorf("resource supposed to be synced but not present: %q", id)
			}
		}
	}

	t.Run("sync adds and GCs resources", func(t *testing.T) {
		kube, _ := setup(t)

		// without GC on, resources persist if they are not mentioned in subsequent syncs.
		test(t, kube, "", "", false)
		test(t, kube, defs1, defs1, false)
		test(t, kube, defs1+defs2, defs1+defs2, false)
		test(t, kube, defs3, defs1+defs2+defs3, false)

		// Now with GC switched on. That means if we don't include a
		// resource in a sync, it should be deleted.
		kube.GC = true
		test(t, kube, defs2+defs3, defs3+defs2, false)
		test(t, kube, defs1+defs2, defs1+defs2, false)
		test(t, kube, "", "", false)
	})

	t.Run("sync won't doesn't delete if apply failed", func(t *testing.T) {
		kube, _ := setup(t)
		kube.GC = true

		const defs1invalid = `
apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: foobar
  name: dep1
  annotations:
    error: fail to apply this
`
		test(t, kube, defs1, defs1, false)
		test(t, kube, defs1invalid, defs1, true)
	})
}

// ----

// TestApplyOrder checks that applyOrder works as expected.
func TestApplyOrder(t *testing.T) {
	objs := []applyObject{
		{ResourceID: flux.MakeResourceID("test", "Deployment", "deploy")},
		{ResourceID: flux.MakeResourceID("test", "Secret", "secret")},
		{ResourceID: flux.MakeResourceID("", "Namespace", "namespace")},
	}
	sort.Sort(applyOrder(objs))
	for i, name := range []string{"namespace", "secret", "deploy"} {
		_, _, objName := objs[i].ResourceID.Components()
		if objName != name {
			t.Errorf("Expected %q at position %d, got %q", name, i, objName)
		}
	}
}
