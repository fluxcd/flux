package daemon

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"time"

	"github.com/fluxcd/flux/pkg/cluster/kubernetes"

	"github.com/go-kit/kit/log"

	"github.com/fluxcd/flux/pkg/event"

	"github.com/fluxcd/flux/pkg/resource"

	"github.com/argoproj/argo-cd/engine"
	"github.com/argoproj/argo-cd/engine/pkg"
	"github.com/argoproj/argo-cd/engine/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/engine/util/lua"
	settingsutil "github.com/argoproj/argo-cd/engine/util/settings"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	clusterURL = "https://kubernetes.default.svc"
	appLabel   = "fluxcd.io/application"
)

func NewEngine(
	namespace string,
	repoURL string,
	daemon *Daemon,
	syncInterval time.Duration,
	prune bool,
	logger log.Logger,
) (pkg.Engine, error) {
	// In-memory sync tag state
	ratchet := &lastKnownSyncState{logger: logger, state: daemon.SyncState}

	a := &engineAdaptor{namespace: namespace, daemon: daemon, ratchet: ratchet}
	reconciliationSettings := &settingsutil.StaticReconciliationSettings{
		AppInstanceLabelKey: appLabel,
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
				TargetRevision: daemon.GitConfig.Branch,
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
	return engine.NewEngine(namespace, reconciliationSettings, creds, a, appclient, a, a, syncInterval, syncInterval, 9999, 20, func() error {
		return nil
	}, func(overrides map[string]v1alpha1.ResourceOverride) *lua.VM {
		return &lua.VM{
			ResourceOverrides: overrides,
		}
	}, a)
}

type engineAdaptor struct {
	namespace string
	daemon    *Daemon
	ratchet   *lastKnownSyncState
}

func (a *engineAdaptor) Generate(ctx context.Context, repo *v1alpha1.Repository, revision string, source *v1alpha1.ApplicationSource, setting *pkg.ManifestGenerationSettings) (*pkg.ManifestResponse, error) {
	resolvedRevision, err := a.daemon.Repo.Revision(ctx, revision)
	if err != nil {
		return nil, err
	}
	resources, err := a.daemon.GetManifests(ctx, revision)
	if err != nil {
		return nil, err
	}

	syncSetName := makeGitConfigHash(a.daemon.Repo.Origin(), a.daemon.GitConfig)
	mfst := make([]string, 0)
	for _, res := range resources {
		csum := sha1.Sum(res.Bytes())
		checkHex := hex.EncodeToString(csum[:])
		data, err := kubernetes.ApplyMetadata(res, syncSetName, checkHex, map[string]string{appLabel: "flux"})
		if err != nil {
			return nil, err
		}

		data, err = yaml.YAMLToJSON(data)
		if err != nil {
			return nil, err
		}
		mfst = append(mfst, string(data))
	}
	return &pkg.ManifestResponse{Namespace: a.namespace, Revision: resolvedRevision, Manifests: mfst}, nil
}

func (a *engineAdaptor) OnBeforeSync(appName string, tasks []pkg.SyncTaskInfo) ([]pkg.SyncTaskInfo, error) {
	syncSetName := makeGitConfigHash(a.daemon.Repo.Origin(), a.daemon.GitConfig)
	res := make([]pkg.SyncTaskInfo, 0)
	for _, task := range tasks {
		gc := task.TargetObj == nil && task.LiveObj != nil
		unsafeGC := gc && !kubernetes.AllowedForGC(task.LiveObj, syncSetName)
		if !unsafeGC {
			res = append(res, task)
		}
	}
	return res, nil
}

func (a *engineAdaptor) OnSyncCompleted(appName string, op v1alpha1.OperationState) error {
	resourceIDs := resource.IDSet{}
	resourceErrors := make([]event.ResourceError, 0)
	for _, res := range op.SyncResult.Resources {
		id := resource.MakeID(res.Namespace, res.Kind, res.Name)
		switch res.Status {
		case v1alpha1.ResultCodeSynced:
			resourceIDs.Add([]resource.ID{id})
		case v1alpha1.ResultCodeSyncFailed:
			// TODO: Argo CD does not preserve resource source path. We need to support it
			resourceErrors = append(resourceErrors, event.ResourceError{ID: id, Error: res.Message})
		}
	}

	return a.daemon.PostSync(context.Background(), op.StartedAt.Time, op.SyncResult.Revision, resourceIDs, resourceErrors, a.ratchet)
}

func (a *engineAdaptor) LogAppEvent(app *v1alpha1.Application, info pkg.EventInfo, message string) {
}

func (a *engineAdaptor) SetAppResourcesTree(appName string, resourcesTree *v1alpha1.ApplicationTree) error {
	return nil
}

func (a *engineAdaptor) SetAppManagedResources(appName string, managedResources []*v1alpha1.ResourceDiff) error {
	return nil
}

func (a *engineAdaptor) GetAppManagedResources(appName string, res *[]*v1alpha1.ResourceDiff) error {
	return errors.New("not supported")
}
