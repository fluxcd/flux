package sync

import (
	"fmt"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/jobs"
	"github.com/weaveworks/flux/platform"
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
	logger := log.NewContext(s.logger).With("job", job.ID)
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

	inst.Logger = log.NewContext(inst.Logger).With("sync-id", string(job.ID))

	// Fetch and update the git repo
	rc := release.NewReleaseContext(inst)
	if err = rc.CloneRepo(); err != nil {
		return nil, errors.Wrap(err, "cloning repo")
	}
	defer rc.Clean()

	// Find all defined services
	defined, err := rc.FindDefinedServices()
	if err != nil {
		return nil, err
	}

	// TODO: When doing removals, we won't need to intersect this.
	var ids []flux.ServiceID
	for _, s := range defined {
		ids = append(ids, s.ServiceID)
	}

	// Fetch the running configs
	running, err := rc.Instance.GetServices(ids)
	if err != nil {
		return nil, err
	}

	runningByID := map[flux.ServiceID]platform.Service{}
	for _, r := range running {
		runningByID[r.ID] = r
	}

	// For each defined config
	var toFix []platform.ServiceDefinition
	for _, d := range defined {
		result, err := s.Diff(d, runningByID[d.ServiceID])
		if err != nil {
			logger.Log("error", errors.Wrapf(err, "diffing %s", d.ServiceID))
			continue
		}

		// If there is a diff
		if result != nil {
			// Create a diff event, to record it
			// Schedule the diff to be fixed.
			toFix = append(toFix, platform.ServiceDefinition{
				ServiceID:     d.ServiceID,
				NewDefinition: d.ManifestBytes,
			})
			continue
		}
	}

	// Fix all the diffs
	results := flux.ReleaseResult{}
	transactionErr := rc.Instance.PlatformApply(toFix)
	if transactionErr != nil {
		switch err := transactionErr.(type) {
		case platform.ApplyError:
			for id, applyErr := range err {
				results[id] = flux.ServiceResult{
					Status: flux.ReleaseStatusFailed,
					Error:  applyErr.Error(),
				}
			}
		default:
			// We assume this means everything failed...
			for _, d := range defined {
				results[d.ServiceID] = flux.ServiceResult{
					Status: flux.ReleaseStatusUnknown,
					Error:  transactionErr.Error(),
				}
			}
		}
	}

	// Update event endTimes for diffs which were fixed
	/*
		for _, d := range defined {
			// Close all outstanding diff events
			result := results[d.ServiceID]
			// If result == nil, it means we missed closing a diff event previously,
			// but that's fine, we should just clean up the events.
		}
	*/

	return nil, fmt.Errorf("TODO: implement sync.Syncer.Handle")
}

func (s *Syncer) Diff(defined *release.ServiceUpdate, running platform.Service) (*Diff, error) {
	return nil, fmt.Errorf("TODO: implement sync.Syncer.Diff")
}
