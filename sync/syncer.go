package sync

import (
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/jobs"
	"github.com/weaveworks/flux/platform"
	"github.com/weaveworks/flux/platform/kubernetes/resource"
	"github.com/weaveworks/flux/release"
)

func NewSyncer(instancer instance.Instancer, logger log.Logger) *Syncer {
	return &Syncer{
		instancer: instancer,
		logger:    logger,
	}
}

type Syncer struct {
	instancer instance.Instancer
	logger    log.Logger
}

func (s *Syncer) Handle(job *jobs.Job, updater jobs.JobUpdater) ([]jobs.Job, error) {
	params := job.Params.(jobs.SyncJobParams)

	inst, err := s.instancer.Get(params.InstanceID)
	if err != nil {
		return nil, err
	}

	config, err := inst.GetConfig()
	if err != nil {
		return nil, errors.Wrap(err, "getting instance config")
	}

	if !config.Settings.Watching {
		return nil, nil
	}

	inst.Logger = log.NewContext(inst.Logger).With("jobID", string(job.ID))
	return nil, sync(inst)
}

func sync(inst *instance.Instance) error {
	// Fetch and update the git repo
	rc := release.NewReleaseContext(inst)
	if err := rc.CloneRepo(); err != nil {
		return errors.Wrap(err, "cloning repo")
	}
	defer rc.Clean()

	// Start rewrite

	// Get a map of resources defined in the repo
	repoResources, err := resource.Load(rc.RepoPath())
	if err != nil {
		return err
	}

	// Get a map of resources defined in the cluster
	clusterBytes, err := inst.Export()
	if err != nil {
		return err
	}
	clusterResources, err := resource.ParseMultidoc(clusterBytes, "exported")
	if err != nil {
		return err
	}

	// Everything that's in the cluster but not in the repo, delete;
	// everything that's in the repo, apply. This is an approximation
	// to figuring out what's changed, and applying that. We're
	// relying on Kubernetes to decide for each resource if it is a
	// no-op.

	// TODO look at ignore notifications
	var sync platform.SyncDef
	report := map[string]string{}
	for id, res := range clusterResources {
		if _, ok := repoResources[id]; !ok {
			sync.Actions = append(sync.Actions, platform.SyncAction{
				ResourceID: id,
				Delete:     res.Bytes(),
			})
		}
		report[id] = "delete"
	}
	for id, res := range repoResources {
		sync.Actions = append(sync.Actions, platform.SyncAction{
			ResourceID: id,
			Apply:      res.Bytes(),
		})
		report[id] = "apply"
	}

	inst.Logger.Log("sync", report)
	// TODO Record event with results?
	// TODO Notification?

	return inst.Platform.Sync(sync)
}
