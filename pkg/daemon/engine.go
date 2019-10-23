package daemon

import (
	"context"
	"fmt"
	"time"

	"github.com/argoproj/argo-cd/engine"
	"github.com/argoproj/argo-cd/engine/pkg"
	"github.com/argoproj/argo-cd/engine/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/engine/util/lua"
	settingsutil "github.com/argoproj/argo-cd/engine/util/settings"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/flux/pkg/git"
	"github.com/fluxcd/flux/pkg/manifests"
)

const (
	clusterURL = "https://kubernetes.default.svc"
)

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
	manifestGenerator := &manifestGenerator{
		manifests:                 manifests,
		repo:                      repo,
		gitTimeout:                gitTimeout,
		gitConfig:                 gitConfig,
		manifestGenerationEnabled: manifestGenerationEnabled,
		gitSecretEnabled:          gitSecretEnabled,
		namespace:                 namespace,
	}

	reconciliationSettings := &settingsutil.StaticReconciliationSettings{
		AppInstanceLabelKey: "fluxcd.io/application",
	}
	creds := &settingsutil.StaticCredsStore{
		Clusters: map[string]v1alpha1.Cluster{clusterURL: {Server: clusterURL}},
		Repos:    map[string]v1alpha1.Repository{repoURL: {Repo: repoURL}},
	}
	appclient := settingsutil.NewStaticAppClientSet(v1alpha1.AppProject{
		ObjectMeta: v1.ObjectMeta{Name: "default", Namespace: namespace},
		Spec: v1alpha1.AppProjectSpec{
			ClusterResourceWhitelist: []v1.GroupKind{{Group: "*", Kind: "*"}},
			Destinations:             []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}},
			SourceRepos:              []string{"*"},
		},
	}, v1alpha1.Application{
		ObjectMeta: v1.ObjectMeta{Name: "flux", Namespace: namespace},
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
	})
	return engine.NewEngine(namespace, reconciliationSettings, creds, &stub{}, appclient, manifestGenerator, &stub{}, syncInterval, syncInterval, 9999, 20, func() error {
		return nil
	}, func(overrides map[string]v1alpha1.ResourceOverride) *lua.VM {
		return &lua.VM{
			ResourceOverrides: overrides,
		}
	})
}

type manifestGenerator struct {
	repo                      *git.Repo
	gitTimeout                time.Duration
	gitSecretEnabled          bool
	gitConfig                 git.Config
	namespace                 string
	manifestGenerationEnabled bool
	manifests                 manifests.Manifests
}

func (a *manifestGenerator) getManifestStore(r repo) (manifests.Store, error) {
	absPaths := git.MakeAbsolutePaths(r, a.gitConfig.Paths)
	if a.manifestGenerationEnabled {
		return manifests.NewConfigAware(r.Dir(), absPaths, a.manifests)
	}
	return manifests.NewRawFiles(r.Dir(), absPaths, a.manifests), nil
}

func (a *manifestGenerator) Generate(ctx context.Context, repo *v1alpha1.Repository, revision string, source *v1alpha1.ApplicationSource, setting *pkg.ManifestGenerationSettings) (*pkg.ManifestResponse, error) {
	// Make a read-only clone used for this sync
	ctxt, cancel := context.WithTimeout(ctx, a.gitTimeout)
	working, err := a.repo.Export(ctxt, revision)
	if err != nil {
		return nil, err
	}
	cancel()
	defer working.Clean()

	// Unseal any secrets if enabled
	if a.gitSecretEnabled {
		ctxt, cancel := context.WithTimeout(ctx, a.gitTimeout)
		if err := working.SecretUnseal(ctxt); err != nil {
			return nil, err
		}
		cancel()
	}
	resourceStore, err := a.getManifestStore(working)
	if err != nil {
		return nil, fmt.Errorf("reading the repository checkout: %v", err)
	}
	ctxt, cancel = context.WithTimeout(ctx, a.gitTimeout)
	defer cancel()
	revision, err = a.repo.Revision(ctxt, revision)
	if err != nil {
		return nil, err
	}

	resources, err := resourceStore.GetAllResourcesByID(ctx)

	mfst := make([]string, 0)
	for i := range resources {
		data, err := yaml.YAMLToJSON(resources[i].Bytes())
		if err != nil {
			return nil, err
		}
		mfst = append(mfst, string(data))
	}
	return &pkg.ManifestResponse{Namespace: a.namespace, Revision: revision, Manifests: mfst}, nil
}

type stub struct {
}

func (s *stub) LogAppEvent(app *v1alpha1.Application, info pkg.EventInfo, message string) {
}

func (s *stub) SetAppResourcesTree(appName string, resourcesTree *v1alpha1.ApplicationTree) error {
	return nil
}

func (s *stub) SetAppManagedResources(appName string, managedResources []*v1alpha1.ResourceDiff) error {
	return nil
}

func (s *stub) GetAppManagedResources(appName string, res *[]*v1alpha1.ResourceDiff) error {
	return errors.New("not supported")
}
