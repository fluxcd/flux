package kubernetes

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	crdfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sclient "k8s.io/client-go/kubernetes"
	corefake "k8s.io/client-go/kubernetes/fake"
	k8s_testing "k8s.io/client-go/testing"

	fhrfake "github.com/fluxcd/flux/integrations/client/clientset/versioned/fake"
	"github.com/fluxcd/flux/pkg/cluster"
	kresource "github.com/fluxcd/flux/pkg/cluster/kubernetes/resource"
	"github.com/fluxcd/flux/pkg/resource"
	"github.com/fluxcd/flux/pkg/sync"
	helmopfake "github.com/fluxcd/helm-operator/pkg/client/clientset/versioned/fake"
)

const (
	defaultTestNamespace = "unusual-default"
)

func fakeClients() (ExtendedClient, func()) {
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
	fhrClient := fhrfake.NewSimpleClientset()
	hrClient := helmopfake.NewSimpleClientset()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	crdClient := crdfake.NewSimpleClientset()
	shutdown := make(chan struct{})
	discoveryClient := MakeCachedDiscovery(coreClient.Discovery(), crdClient, shutdown)

	// Assigned here, since this is _also_ used by the (fake)
	// discovery client therein, and ultimately by
	// getResourcesInStack since that uses the core clientset to
	// enumerate the namespaces.
	coreClient.Fake.Resources = apiResources

	if debug {
		for _, fake := range []*k8s_testing.Fake{&coreClient.Fake, &fhrClient.Fake, &hrClient.Fake, &dynamicClient.Fake} {
			fake.PrependReactor("*", "*", func(action k8s_testing.Action) (bool, runtime.Object, error) {
				gvr := action.GetResource()
				fmt.Printf("[DEBUG] action: %s ns:%s %s/%s %s\n", action.GetVerb(), action.GetNamespace(), gvr.Group, gvr.Version, gvr.Resource)
				return false, nil, nil
			})
		}
	}

	ec := ExtendedClient{
		coreClient:         coreClient,
		fluxHelmClient:     fhrClient,
		helmOperatorClient: hrClient,
		dynamicClient:      dynamicClient,
		discoveryClient:    discoveryClient,
	}

	return ec, func() { close(shutdown) }
}

// fakeApplier is an Applier that just forwards changeset operations
// to a dynamic client. It doesn't try to properly patch resources
// when that might be expected; it just overwrites them. But this is
// enough for checking whether sync operations succeeded and had the
// correct effect, which is either to "upsert", or delete, resources.
type fakeApplier struct {
	dynamicClient dynamic.Interface
	coreClient    k8sclient.Interface
	defaultNS     string
	commandRun    bool
}

func groupVersionResource(res *unstructured.Unstructured) schema.GroupVersionResource {
	gvk := res.GetObjectKind().GroupVersionKind()
	return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: strings.ToLower(gvk.Kind) + "s"}
}

func (a fakeApplier) apply(_ log.Logger, cs changeSet, errored map[resource.ID]error) cluster.SyncError {
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
		c := a.dynamicClient.Resource(gvr)
		// This is an approximation to what `kubectl` does in filling
		// in the fallback namespace (from config). In the case of
		// non-namespaced entities, it will be ignored by the fake
		// client (FIXME: make sure of this).
		apiRes := findAPIResource(gvr, a.coreClient.Discovery())
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
				_, err = dc.Create(res, metav1.CreateOptions{})
			case err == nil:
				_, err = dc.Update(res, metav1.UpdateOptions{})
			}
			if err != nil {
				errs = append(errs, cluster.ResourceError{obj.ResourceID, obj.Source, err})
				return
			}
			if res.GetKind() == "Namespace" {
				// We also create namespaces in the core fake client since the dynamic client
				// and core clients don't share resources
				var ns corev1.Namespace
				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstruct, &ns); err != nil {
					errs = append(errs, cluster.ResourceError{obj.ResourceID, obj.Source, err})
					return
				}
				_, err := a.coreClient.CoreV1().Namespaces().Get(ns.Name, metav1.GetOptions{})
				switch {
				case errors.IsNotFound(err):
					_, err = a.coreClient.CoreV1().Namespaces().Create(&ns)
				case err == nil:
					_, err = a.coreClient.CoreV1().Namespaces().Update(&ns)
				}
				if err != nil {
					errs = append(errs, cluster.ResourceError{obj.ResourceID, obj.Source, err})
					return
				}
			}

		} else if cmd == "delete" {
			if err := dc.Delete(name, &metav1.DeleteOptions{}); err != nil {
				errs = append(errs, cluster.ResourceError{obj.ResourceID, obj.Source, err})
				return
			}
			if res.GetKind() == "Namespace" {
				// We also create namespaces in the core fake client since the dynamic client
				// and core clients don't share resources
				if err := a.coreClient.CoreV1().Namespaces().Delete(res.GetName(), &metav1.DeleteOptions{}); err != nil {
					errs = append(errs, cluster.ResourceError{obj.ResourceID, obj.Source, err})
					return
				}
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

func setup(t *testing.T) (*Cluster, *fakeApplier, func()) {
	clients, cancel := fakeClients()
	applier := &fakeApplier{dynamicClient: clients.dynamicClient, coreClient: clients.coreClient, defaultNS: defaultTestNamespace}
	kube := &Cluster{
		applier:             applier,
		client:              clients,
		logger:              log.NewLogfmtLogger(os.Stdout),
		resourceExcludeList: []string{"*metrics.k8s.io/*", "webhook.certmanager.k8s.io/v1beta1/*"},
	}
	return kube, applier, cancel
}

func TestSyncNop(t *testing.T) {
	kube, mock, cancel := setup(t)
	defer cancel()
	if err := kube.Sync(cluster.SyncSet{}); err != nil {
		t.Errorf("%#v", err)
	}
	if mock.commandRun {
		t.Error("expected no commands run")
	}
}

func TestSyncTolerateEmptyGroupVersion(t *testing.T) {
	kube, _, cancel := setup(t)
	defer cancel()

	// Add a GroupVersion without API Resources
	fakeClient := kube.client.coreClient.(*corefake.Clientset)
	fakeClient.Resources = append(fakeClient.Resources, &metav1.APIResourceList{GroupVersion: "foo.bar/v1"})

	// We should tolerate the error caused in the cache due to the
	// GroupVersion being empty
	err := kube.Sync(cluster.SyncSet{})
	assert.NoError(t, err)

	// No errors the second time either
	err = kube.Sync(cluster.SyncSet{})
	assert.NoError(t, err)
}

type failingDiscoveryClient struct {
	discovery.DiscoveryInterface
}

func (d *failingDiscoveryClient) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	return nil, errors.NewServiceUnavailable("")
}

func TestSyncTolerateMetricsErrors(t *testing.T) {
	kube, _, cancel := setup(t)

	// Replace the discovery client by one returning errors when asking for resources
	cancel()
	crdClient := crdfake.NewSimpleClientset()
	shutdown := make(chan struct{})
	defer close(shutdown)
	newDiscoveryClient := &failingDiscoveryClient{kube.client.coreClient.Discovery()}
	kube.client.discoveryClient = MakeCachedDiscovery(newDiscoveryClient, crdClient, shutdown)

	// Check that syncing results in an error for groups other than metrics
	fakeClient := kube.client.coreClient.(*corefake.Clientset)
	fakeClient.Resources = []*metav1.APIResourceList{{GroupVersion: "foo.bar/v1"}}
	err := kube.Sync(cluster.SyncSet{})
	assert.Error(t, err)

	// Check that syncing doesn't result in an error for a metrics group
	kube.client.discoveryClient.(*cachedDiscovery).CachedDiscoveryInterface.Invalidate()
	fakeClient.Resources = []*metav1.APIResourceList{{GroupVersion: "custom.metrics.k8s.io/v1"}}
	err = kube.Sync(cluster.SyncSet{})
	assert.NoError(t, err)

	kube.client.discoveryClient.(*cachedDiscovery).CachedDiscoveryInterface.Invalidate()
	fakeClient.Resources = []*metav1.APIResourceList{{GroupVersion: "webhook.certmanager.k8s.io/v1beta1"}}
	err = kube.Sync(cluster.SyncSet{})
	assert.NoError(t, err)
}

func TestSync(t *testing.T) {
	const ns1 = `---
apiVersion: v1
kind: Namespace
metadata:
  name: foobar
`

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

	const ns3 = `---
apiVersion: v1
kind: Namespace
metadata:
  name: other
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

	testDefaultNs := func(t *testing.T, kube *Cluster, defs, expectedAfterSync string, expectErrors bool, defaultNamespace string) {
		saved := getKubeconfigDefaultNamespace
		getKubeconfigDefaultNamespace = func() (string, error) { return defaultTestNamespace, nil }
		defer func() { getKubeconfigDefaultNamespace = saved }()
		namespacer, err := NewNamespacer(kube.client.coreClient.Discovery(), defaultNamespace)
		if err != nil {
			t.Fatal(err)
		}
		manifests := NewManifests(namespacer, log.NewLogfmtLogger(os.Stdout))

		resources0, err := kresource.ParseMultidoc([]byte(defs), "before")
		if err != nil {
			t.Fatal(err)
		}

		// Needed to get from KubeManifest to resource.Resource
		resources, err := manifests.setEffectiveNamespaces(resources0)
		if err != nil {
			t.Fatal(err)
		}
		resourcesByID := map[string]resource.Resource{}
		for _, r := range resources {
			resourcesByID[r.ResourceID().String()] = r
		}
		err = sync.Sync("testset", resourcesByID, kube)
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
	test := func(t *testing.T, kube *Cluster, defs, expectedAfterSync string, expectErrors bool) {
		testDefaultNs(t, kube, defs, expectedAfterSync, expectErrors, "")
	}

	t.Run("sync adds and GCs resources", func(t *testing.T) {
		kube, _, cancel := setup(t)
		defer cancel()

		// without GC on, resources persist if they are not mentioned in subsequent syncs.
		test(t, kube, "", "", false)
		test(t, kube, ns1+defs1, ns1+defs1, false)
		test(t, kube, ns1+defs1+defs2, ns1+defs1+defs2, false)
		test(t, kube, ns3+defs3, ns1+defs1+defs2+ns3+defs3, false)

		// Now with GC switched on. That means if we don't include a
		// resource in a sync, it should be deleted.
		kube.GC = true
		test(t, kube, ns1+defs2+ns3+defs3, ns1+defs2+ns3+defs3, false)
		test(t, kube, ns1+defs1+defs2, ns1+defs1+defs2, false)
		test(t, kube, "", "", false)
	})

	t.Run("sync adds and GCs dry run", func(t *testing.T) {
		kube, _, cancel := setup(t)
		defer cancel()

		// without GC on, resources persist if they are not mentioned in subsequent syncs.
		test(t, kube, "", "", false)
		test(t, kube, ns1+defs1, ns1+defs1, false)
		test(t, kube, ns1+defs1+defs2, ns1+defs1+defs2, false)
		test(t, kube, ns3+defs3, ns1+defs1+defs2+ns3+defs3, false)

		// with GC dry run the collect garbage routine is running but only logging results with out collecting any resources
		kube.DryGC = true
		test(t, kube, ns1+defs2+ns3+defs3, ns1+defs1+defs2+ns3+defs3, false)
		test(t, kube, ns1+defs1+defs2, ns1+defs1+defs2+ns3+defs3, false)
		test(t, kube, "", ns1+defs1+defs2+ns3+defs3, false)
	})

	t.Run("sync won't incorrectly delete non-namespaced resources", func(t *testing.T) {
		kube, _, cancel := setup(t)
		defer cancel()
		kube.GC = true

		const nsDef = `
apiVersion: v1
kind: Namespace
metadata:
  name: bar-ns
`
		test(t, kube, nsDef, nsDef, false)
	})

	t.Run("sync applies default namespace", func(t *testing.T) {
		kube, _, cancel := setup(t)
		defer cancel()
		kube.GC = true

		const depDef = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bar
`
		const depDefNamespaced = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bar
  namespace: system
`
		const depDefAlreadyNamespaced = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bar
  namespace: other
`
		const ns1 = `---
apiVersion: v1
kind: Namespace
metadata:
  name: foobar
`
		defaultNs := "system"
		testDefaultNs(t, kube, depDef, depDefNamespaced, false, defaultNs)
		testDefaultNs(t, kube, depDefAlreadyNamespaced, depDefAlreadyNamespaced, false, defaultNs)
		testDefaultNs(t, kube, ns1, ns1, false, defaultNs)
	})

	t.Run("sync won't delete resources that got the fallback namespace when created", func(t *testing.T) {
		// NB: this tests the fake client implementation to some
		// extent as well. It relies on it to reflect the kubectl
		// behaviour of giving things that need a namespace some
		// fallback (this would come from kubeconfig usually); and,
		// for things that _don't_ have a namespace to have it
		// stripped out.
		kube, _, cancel := setup(t)
		defer cancel()
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
		kube, _, cancel := setup(t)
		defer cancel()
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
		test(t, kube, ns1+dep, ns1+dep, false)

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
		depCopyActual, err := client.Create(depCopy, metav1.CreateOptions{})
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
		kube, _, cancel := setup(t)
		defer cancel()
		kube.GC = true

		const defs1invalid = `---
apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: foobar
  name: dep1
  annotations:
    error: fail to apply this
`
		test(t, kube, ns1+defs1, ns1+defs1, false)
		test(t, kube, ns1+defs1invalid, ns1+defs1invalid, true)
	})

	t.Run("sync doesn't apply or delete manifests marked with ignore", func(t *testing.T) {
		kube, _, cancel := setup(t)
		defer cancel()
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
		test(t, kube, ns1+dep1+dep2, ns1+dep1, false)

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
		test(t, kube, ns1+dep1ignored+dep2, ns1+dep1, false)
	})

	t.Run("sync doesn't update a cluster resource marked with ignore", func(t *testing.T) {
		const dep1 = `---
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
		kube, _, cancel := setup(t)
		defer cancel()
		// This just checks the starting assumption: dep1 exists in the cluster
		test(t, kube, ns1+dep1, ns1+dep1, false)

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
		if _, err = rc.Namespace("foobar").Update(res, metav1.UpdateOptions{}); err != nil {
			t.Fatal(err)
		}

		const mod1 = `---
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
		test(t, kube, ns1+mod1, ns1+dep1, false)
	})

	t.Run("sync doesn't update or delete a pre-existing resource marked with ignore", func(t *testing.T) {
		kube, _, cancel := setup(t)
		defer cancel()

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
		var ns1obj corev1.Namespace
		err = yaml.Unmarshal([]byte(ns1), &ns1obj)
		assert.NoError(t, err)
		// Put the pre-existing resource in the cluster
		_, err = kube.client.coreClient.CoreV1().Namespaces().Create(&ns1obj)
		assert.NoError(t, err)
		dc := kube.client.dynamicClient.Resource(gvr).Namespace(dep1res.GetNamespace())
		_, err = dc.Create(dep1res, metav1.CreateOptions{})
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
		{ResourceID: resource.MakeID("test", "Deployment", "deploy")},
		{ResourceID: resource.MakeID("test", "Secret", "secret")},
		{ResourceID: resource.MakeID("", "Namespace", "namespace")},
	}
	sort.Sort(applyOrder(objs))
	for i, name := range []string{"namespace", "secret", "deploy"} {
		_, _, objName := objs[i].ResourceID.Components()
		if objName != name {
			t.Errorf("Expected %q at position %d, got %q", name, i, objName)
		}
	}
}
