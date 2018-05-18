package release

import (
	"time"

	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux/resource"
	"github.com/weaveworks/flux/update"
)

type Changes interface {
	CalculateRelease(update.ReleaseContext, log.Logger) ([]*update.ControllerUpdate, update.Result, error)
	ReleaseKind() update.ReleaseKind
	ReleaseType() update.ReleaseType
	CommitMessage(update.Result) string
}

func Release(rc *ReleaseContext, changes Changes, logger log.Logger) (results update.Result, err error) {
	defer func(start time.Time) {
		update.ObserveRelease(
			start,
			err == nil,
			changes.ReleaseType(),
			changes.ReleaseKind(),
		)
	}(time.Now())

	logger = log.With(logger, "type", "release")

	before, err := rc.manifests.LoadManifests(rc.repo.Dir(), rc.repo.ManifestDir())
	updates, results, err := changes.CalculateRelease(rc, logger)
	if err != nil {
		return nil, err
	}

	err = ApplyChanges(rc, updates, logger)
	if err == nil {
		var after map[string]resource.Resource
		after, err = rc.manifests.LoadManifests(rc.repo.Dir(), rc.repo.ManifestDir())
		if err == nil {
			err = VerifyChanges(before, updates, after)
		}
	}
	return results, err
}

func ApplyChanges(rc *ReleaseContext, updates []*update.ControllerUpdate, logger log.Logger) error {
	logger.Log("updates", len(updates))
	if len(updates) == 0 {
		logger.Log("exit", "no images to update for services given")
		return nil
	}

	timer := update.NewStageTimer("write_changes")
	err := rc.WriteUpdates(updates)
	timer.ObserveDuration()
	return err
}

func VerifyChanges(before map[string]resource.Resource, updates []*update.ControllerUpdate, after map[string]resource.Resource) error {
	return nil
}
