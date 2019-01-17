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
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/discovery"
	corefake "k8s.io/client-go/kubernetes/fake"
	k8s_testing "k8s.io/client-go/testing"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	kresource "github.com/weaveworks/flux/cluster/kubernetes/resource"
	fluxfake "github.com/weaveworks/flux/integrations/client/clientset/versioned/fake"
	"github.com/weaveworks/flux/sync"
)

const (
	defaultTestNamespace = "unusual-default"
)

func fakeClients() ExtendedClient {
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
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "namespaces", SingularName: "namespace", Namespaced: false, Kind: "Namespace", Verbs: getAndList},
			},
		},
	}

	coreClient := corefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: defaultTestNamespace}})
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
				fmt.Printf("[DEBUG] action: %s ns:%s %s/%s %s\n", action.GetVerb(), action.GetNamespace(), gvr.Group, gvr.Version, gvr.Resource)
				return false, nil, nil
			})
		}
	}

	return ExtendedClient{
		coreClient:      coreClient,
		fluxHelmClient:  fluxClient,
		dynamicClient:   dynamicClient,
		discoveryClient: coreClient.Discovery(),
	}
}

// fakeApplier is an Applier that just forwards changeset operations
// to a dynamic client. It doesn't try to properly patch resources
// when that might be expected; it just overwrites them. But this is
// enough for checking whether sync operations succeeded and had the
// correct effect, which is either to "upsert", or delete, resources.
type fakeApplier struct {
	client     dynamic.Interface
	discovery  discovery.DiscoveryInterface
	defaultNS  string
	commandRun bool
}

func groupVersionResource(res *unstructured.Unstructured) schema.GroupVersionResource {
	gvk := res.GetObjectKind().GroupVersionKind()
	return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: strings.ToLower(gvk.Kind) + "s"}
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

		gvr := groupVersionResource(res)
		c := a.client.Resource(gvr)
		// This is an approximation to what `kubectl` does in filling
		// in the fallback namespace (from config). In the case of
		// non-namespaced entities, it will be ignored by the fake
		// client (FIXME: make sure of this).
		apiRes := findAPIResource(gvr, a.discovery)
		if apiRes == nil {
			panic("no APIResource found for " + gvr.String())
		}

		var dc dynamic.ResourceInterface = c
		ns := res.GetNamespace()
		if apiRes.Namespaced {
			if ns == "" {
				ns = a.defaultNS
				res.SetNamespace(ns)
			}
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

func findAPIResource(gvr schema.GroupVersionResource, disco discovery.DiscoveryInterface) *metav1.APIResource {
	groupVersion := gvr.Version
	if gvr.Group != "" {
		groupVersion = gvr.Group + "/" + groupVersion
	}
	reses, err := disco.ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		return nil
	}
	for _, res := range reses.APIResources {
		if res.Name == gvr.Resource {
			return &res
		}
	}
	return nil
}

// ---

func setup(t *testing.T) (*Cluster, *fakeApplier) {
	clients := fakeClients()
	applier := &fakeApplier{client: clients.dynamicClient, discovery: clients.coreClient.Discovery(), defaultNS: defaultTestNamespace}
	kube := &Cluster{
		applier: applier,
		client:  clients,
		logger:  log.NewNopLogger(),
	}
	return kube, applier
}

func TestSyncNop(t *testing.T) {
	kube, mock := setup(t)
	if err := kube.Sync(cluster.SyncSet{}); err != nil {
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

	// checkSame is a check that a result returned from the cluster is
	// the same as an expected.  labels and annotations may be altered
	// by the sync process; we'll look at the "spec" field as an
	// indication of whether the resources are equivalent or not.
	checkSame := func(t *testing.T, expected []byte, actual *unstructured.Unstructured) {
		var expectedSpec struct{ Spec map[string]interface{} }
		if err := yaml.Unmarshal(expected, &expectedSpec); err != nil {
			t.Error(err)
			return
		}
		if expectedSpec.Spec != nil {
			assert.Equal(t, expectedSpec.Spec, actual.Object["spec"])
		}
	}

	test := func(t *testing.T, kube *Cluster, defs, expectedAfterSync string, expectErrors bool) {
		saved := getDefaultNamespace
		getDefaultNamespace = func() (string, error) { return defaultTestNamespace, nil }
		defer func() { getDefaultNamespace = saved }()
		namespacer, err := NewNamespacer(kube.client.coreClient.Discovery())
		if err != nil {
			t.Fatal(err)
		}

		resources0, err := kresource.ParseMultidoc([]byte(defs), "before")
		if err != nil {
			t.Fatal(err)
		}

		// Needed to get from KubeManifest to resource.Resource
		resources, err := postProcess(resources0, namespacer)
		if err != nil {
			t.Fatal(err)
		}

		err = sync.Sync("testset", resources, kube)
		if !expectErrors && err != nil {
			t.Error(err)
		}
		expected, err := kresource.ParseMultidoc([]byte(expectedAfterSync), "after")
		if err != nil {
			panic(err)
		}

		// Now check that the resources were created
		actual, err := kube.getAllowedGCMarkedResourcesInSyncSet("testset")
		if err != nil {
			t.Fatal(err)
		}

		for id := range actual {
			if _, ok := expected[id]; !ok {
				t.Errorf("resource present after sync but not in resources applied: %q (present: %v)", id, actual)
				if j, err := yaml.Marshal(actual[id].obj); err == nil {
					println(string(j))
				}
				continue
			}
			checkSame(t, expected[id].Bytes(), actual[id].obj)
		}
		for id := range expected {
			if _, ok := actual[id]; !ok {
				t.Errorf("resource supposed to be synced but not present: %q (present: %v)", id, actual)
			}
			// no need to compare values, since we already considered
			// the intersection of actual and expected above.
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

	t.Run("sync won't incorrectly delete non-namespaced resources", func(t *testing.T) {
		kube, _ := setup(t)
		kube.GC = true

		const nsDef = `
apiVersion: v1
kind: Namespace
metadata:
  name: bar-ns
`
		test(t, kube, nsDef, nsDef, false)
	})

	t.Run("sync won't delete resources that got the fallback namespace when created", func(t *testing.T) {
		// NB: this tests the fake client implementation to some
		// extent as well. It relies on it to reflect the kubectl
		// behaviour of giving things that need a namespace some
		// fallback (this would come from kubeconfig usually); and,
		// for things that _don't_ have a namespace to have it
		// stripped out.
		kube, _ := setup(t)
		kube.GC = true
		const withoutNS = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: depFallbackNS
`
		const withNS = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: depFallbackNS
  namespace: ` + defaultTestNamespace + `
`
		test(t, kube, withoutNS, withNS, false)
	})

	t.Run("sync won't delete resources whose garbage collection mark was copied to", func(t *testing.T) {
		kube, _ := setup(t)
		kube.GC = true

		depName := "dep"
		depNS := "foobar"
		dep := fmt.Sprintf(`---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
`, depName, depNS)

		// Add dep to the cluster through syncing
		test(t, kube, dep, dep, false)

		// Add a copy of dep (including the GCmark label) with different name directly to the cluster
		gvr := schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1",
			Resource: "deployments",
		}
		client := kube.client.dynamicClient.Resource(gvr).Namespace(depNS)
		depActual, err := client.Get(depName, metav1.GetOptions{})
		assert.NoError(t, err)
		depCopy := depActual.DeepCopy()
		depCopyName := depName + "copy"
		depCopy.SetName(depCopyName)
		depCopyActual, err := client.Create(depCopy)
		assert.NoError(t, err)

		// Check that both dep and its copy have the same GCmark label
		assert.Equal(t, depActual.GetName()+"copy", depCopyActual.GetName())
		assert.NotEmpty(t, depActual.GetLabels()[gcMarkLabel])
		assert.Equal(t, depActual.GetLabels()[gcMarkLabel], depCopyActual.GetLabels()[gcMarkLabel])

		// Remove defs1  from the cluster through syncing
		test(t, kube, "", "", false)

		// Check that defs1 is removed from the cluster but its copy isn't, due to having a different name
		_, err = client.Get(depName, metav1.GetOptions{})
		assert.Error(t, err)
		_, err = client.Get(depCopyName, metav1.GetOptions{})
		assert.NoError(t, err)
	})

	t.Run("sync won't delete if apply failed", func(t *testing.T) {
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

	t.Run("sync doesn't apply or delete manifests marked with ignore", func(t *testing.T) {
		kube, _ := setup(t)
		kube.GC = true

		const dep1 = `---
apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: foobar
  name: dep1
spec:
  metadata:
    labels: {app: foo}
`

		const dep2 = `---
apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: foobar
  name: dep2
  annotations: {flux.weave.works/ignore: "true"}
`

		// dep1 is created, but dep2 is ignored
		test(t, kube, dep1+dep2, dep1, false)

		const dep1ignored = `---
apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: foobar
  name: dep1
  annotations:
    flux.weave.works/ignore: "true"
spec:
  metadata:
    labels: {app: bar}
`
		// dep1 is not updated, but neither is it deleted
		test(t, kube, dep1ignored+dep2, dep1, false)
	})

	t.Run("sync doesn't update a cluster resource marked with ignore", func(t *testing.T) {
		const dep1 = `
apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: foobar
  name: dep1
spec:
  metadata:
    labels:
      app: original
`
		kube, _ := setup(t)
		// This just checks the starting assumption: dep1 exists in the cluster
		test(t, kube, dep1, dep1, false)

		// Now we'll mark it as ignored _in the cluster_ (i.e., the
		// equivalent of `kubectl annotate`)
		dc := kube.client.dynamicClient
		rc := dc.Resource(schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1",
			Resource: "deployments",
		})
		res, err := rc.Namespace("foobar").Get("dep1", metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		annots := res.GetAnnotations()
		annots["flux.weave.works/ignore"] = "true"
		res.SetAnnotations(annots)
		if _, err = rc.Namespace("foobar").Update(res); err != nil {
			t.Fatal(err)
		}

		const mod1 = `
apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: foobar
  name: dep1
spec:
  metadata:
    labels:
      app: modified
`
		// Check that dep1, which is marked ignore in the cluster, is
		// neither updated or deleted
		test(t, kube, mod1, dep1, false)
	})

	t.Run("sync doesn't update or delete a pre-existing resource marked with ignore", func(t *testing.T) {
		kube, _ := setup(t)

		const existing = `---
apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: foobar
  name: dep1
  annotations: {flux.weave.works/ignore: "true"}
spec:
  metadata:
    labels: {foo: original}
`
		var dep1obj map[string]interface{}
		err := yaml.Unmarshal([]byte(existing), &dep1obj)
		assert.NoError(t, err)
		dep1res := &unstructured.Unstructured{Object: dep1obj}
		gvr := groupVersionResource(dep1res)
		// Put the pre-existing resource in the cluster
		dc := kube.client.dynamicClient.Resource(gvr).Namespace(dep1res.GetNamespace())
		_, err = dc.Create(dep1res)
		assert.NoError(t, err)

		// Check that our resource-getting also sees the pre-existing resource
		resources, err := kube.getAllowedResourcesBySelector("")
		assert.NoError(t, err)
		assert.Contains(t, resources, "foobar:deployment/dep1")

		// NB test checks the _synced_ resources, so this just asserts
		// the precondition, that nothing is synced
		test(t, kube, "", "", false)

		// .. but, our resource is still there.
		r, err := dc.Get(dep1res.GetName(), metav1.GetOptions{})
		assert.NoError(t, err)
		assert.NotNil(t, r)

		const update = `---
apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: foobar
  name: dep1
spec:
  metadata:
    labels: {foo: modified}
`

		// Check that it's not been synced (i.e., still not included in synced resources)
		test(t, kube, update, "", false)

		// Check that it still exists, as created
		r, err = dc.Get(dep1res.GetName(), metav1.GetOptions{})
		assert.NoError(t, err)
		assert.NotNil(t, r)
		checkSame(t, []byte(existing), r)
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
