package kubernetes

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/argoproj/argo-cd/engine/pkg/utils/kube/cache"
	"github.com/argoproj/argo-cd/engine/pkg/utils/kube/sync"
	"github.com/argoproj/argo-cd/engine/pkg/utils/kube/sync/common"

	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	corefake "k8s.io/client-go/kubernetes/fake"
	k8s_testing "k8s.io/client-go/testing"

	fhrfake "github.com/fluxcd/flux/integrations/client/clientset/versioned/fake"
	"github.com/fluxcd/flux/pkg/cluster"
	helmopfake "github.com/fluxcd/helm-operator/pkg/client/clientset/versioned/fake"
)

const (
	defaultTestNamespace = "unusual-default"
)

type fakeEngine struct {
}

func (e *fakeEngine) Run() (io.Closer, error) {
	return nil, nil
}
func (e *fakeEngine) Sync(ctx context.Context, resources []*unstructured.Unstructured, isManaged func(r *cache.Resource) bool, revision string, namespace string, opts ...sync.SyncOpt) ([]common.ResourceSyncResult, error) {
	return nil, nil
}

func fakeClients() (ExtendedClient, func()) {
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
	shutdown := make(chan struct{})

	// Assigned here, since this is _also_ used by the (fake)
	// discovery client therein, and ultimately by
	// getResourcesInStack since that uses the core clientset to
	// enumerate the namespaces.
	coreClient.Fake.Resources = apiResources

	if debug {
		for _, fake := range []*k8s_testing.Fake{&coreClient.Fake, &fhrClient.Fake, &hrClient.Fake} {
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
	}

	return ec, func() { close(shutdown) }
}

// ---

func setup(t *testing.T) (*Cluster, func()) {
	clients, cancel := fakeClients()
	kube := &Cluster{
		engine: &fakeEngine{},
		client: clients,
		logger: log.NewLogfmtLogger(os.Stdout),
	}
	return kube, cancel
}

func TestSyncTolerateEmptyGroupVersion(t *testing.T) {
	kube, cancel := setup(t)
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
