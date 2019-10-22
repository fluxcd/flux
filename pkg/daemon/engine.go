package daemon

import (
	"encoding/json"
	"time"

	"k8s.io/apimachinery/pkg/util/strategicpatch"

	"github.com/fluxcd/flux/pkg/manifests"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/testing"

	"github.com/argoproj/argo-cd/engine"
	"github.com/argoproj/argo-cd/engine/pkg"
	"github.com/argoproj/argo-cd/engine/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/engine/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/engine/util/lua"
	"github.com/fluxcd/flux/pkg/git"
)

const (
	clusterURL = "https://kubernetes.default.svc"
)

type engineSettings struct {
	repo                      *git.Repo
	gitTimeout                time.Duration
	gitSecretEnabled          bool
	gitConfig                 git.Config
	namespace                 string
	manifestGenerationEnabled bool
	manifests                 manifests.Manifests
}

func NewEngine(
	namespace string,
	repoURL string,
	repo *git.Repo,
	gitTimeout time.Duration,
	gitSecretEnabled bool,
	gitConfig git.Config,
	manifestGenerationEnabled bool,
	manifests manifests.Manifests,
	syncInterval time.Duration,
	prune bool,
) (pkg.Engine, error) {

	settings := &engineSettings{
		manifests:                 manifests,
		repo:                      repo,
		gitTimeout:                gitTimeout,
		gitConfig:                 gitConfig,
		manifestGenerationEnabled: manifestGenerationEnabled,
		gitSecretEnabled:          gitSecretEnabled,
		namespace:                 namespace,
	}

	// TODO: stop using fake client set and provider proper implementation
	clientset := appclientset.NewSimpleClientset(&v1alpha1.Application{
		ObjectMeta: v1.ObjectMeta{
			Name:      "flux",
			Namespace: namespace,
		},
		Spec: v1alpha1.ApplicationSpec{
			Source: v1alpha1.ApplicationSource{
				RepoURL:        repoURL,
				Path:           ".",
				TargetRevision: gitConfig.Branch,
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    clusterURL,
				Namespace: namespace,
			},
			SyncPolicy: &v1alpha1.SyncPolicy{
				Automated: &v1alpha1.SyncPolicyAutomated{
					Prune:    prune,
					SelfHeal: true,
				},
			},
			Project: "default",
		},
	}, &v1alpha1.AppProject{
		ObjectMeta: v1.ObjectMeta{
			Name:      "default",
			Namespace: namespace,
		},
		Spec: v1alpha1.AppProjectSpec{
			ClusterResourceWhitelist: []v1.GroupKind{{Group: "*", Kind: "*"}},
			Destinations:             []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}},
			SourceRepos:              []string{"*"},
		},
	})

	clientset.PrependReactor("patch", "applications", func(action testing.Action) (handled bool, ret runtime.Object, err error) {
		patchAction, ok := action.(testing.PatchActionImpl)
		if !ok {
			return false, nil, nil
		}
		clientset.Unlock()
		defer clientset.Lock()
		app, err := clientset.ArgoprojV1alpha1().Applications(patchAction.Namespace).Get(patchAction.Name, v1.GetOptions{})
		if err != nil {
			return false, nil, err
		}

		origBytes, err := json.Marshal(app)
		if err != nil {
			return false, nil, err
		}
		newAppData, err := strategicpatch.StrategicMergePatch(origBytes, patchAction.Patch, app)
		if err != nil {
			return false, nil, err
		}
		updatedApp := &v1alpha1.Application{}
		err = json.Unmarshal(newAppData, updatedApp)
		if err != nil {
			return false, nil, err
		}
		updatedApp, err = clientset.ArgoprojV1alpha1().Applications(patchAction.Namespace).Update(updatedApp)
		if err != nil {
			return false, nil, err
		}

		return true, updatedApp, nil
	})

	return engine.NewEngine(namespace, settings, settings, settings, clientset, settings, settings, syncInterval, syncInterval, 9999, 20, func() error {
		return nil
	}, func(overrides map[string]v1alpha1.ResourceOverride) *lua.VM {
		return &lua.VM{
			ResourceOverrides: overrides,
		}
	})
}
