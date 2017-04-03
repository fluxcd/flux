package sync

import (
	"fmt"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/jobs"
	"github.com/weaveworks/flux/platform"
	"github.com/weaveworks/flux/platform/kubernetes/resource"
	"github.com/weaveworks/flux/release"
)

func NewSyncer(instancer instance.Instancer, instanceDB instance.DB, logger log.Logger) *Syncer {
	return &Syncer{
		instancer:  instancer,
		instanceDB: instanceDB,
		logger:     logger,
	}
}

type Syncer struct {
	instancer  instance.Instancer
	instanceDB instance.DB
	logger     log.Logger
}

func (s *Syncer) Handle(job *jobs.Job, updater jobs.JobUpdater) ([]jobs.Job, error) {
	params := job.Params.(jobs.SyncJobParams)

	config, err := s.instanceDB.GetConfig(params.InstanceID)
	if err != nil {
		return nil, errors.Wrap(err, "getting instance config")
	}

	if !config.Settings.Watching {
		return nil, nil
	}

	inst, err := s.instancer.Get(params.InstanceID)
	if err != nil {
		return nil, err
	}

	inst.Logger = log.NewContext(inst.Logger).With("jobID", string(job.ID))

	// Fetch and update the git repo
	rc := release.NewReleaseContext(inst)
	if err = rc.CloneRepo(); err != nil {
		return nil, errors.Wrap(err, "cloning repo")
	}
	defer rc.Clean()

	// Start rewrite

	// Get a map of resources defined in the repo
	repoResources, err := resource.Load(rc.RepoPath())
	if err != nil {
		return nil, err
	}

	// Get a map of resources defined in the cluster
	clusterBytes, err := inst.Export()
	if err != nil {
		return nil, err
	}
	clusterResources, err := resource.ParseMultidoc(clusterBytes, "exported")
	if err != nil {
		return nil, err
	}

	// Everything that's in the cluster but not in the repo, delete;
	// everything that's in the repo, apply. This is an approximation
	// to figuring out what's changed, and applying that. We're
	// relying on Kubernetes to decide for each application is it is a
	// no-op.
	var sync platform.SyncDef
	for id, res := range clusterResources {
		if _, ok := repoResources[id]; !ok {
			sync.Actions = append(sync.Actions, platform.SyncAction{
				ResourceID: id,
				Delete:     res.Bytes(),
			})
		}
	}
	for id, res := range repoResources {
		sync.Actions = append(sync.Actions, platform.SyncAction{
			ResourceID: id,
			Apply:      res.Bytes(),
		})
	}

	// TODO log something?
	// TODO Record event with results?
	// TODO Notification?

	return nil, inst.Platform.Sync(sync)
}

func (s *Syncer) Diff(defined *release.ServiceUpdate, running platform.Service) (*Diff, error) {
	return nil, fmt.Errorf("TODO: implement sync.Syncer.Diff")
}
