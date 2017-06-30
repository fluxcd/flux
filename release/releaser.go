package release

import (
	"time"

	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux/update"
)

type Changes interface {
	CalculateRelease(update.ReleaseContext, log.Logger) ([]*update.ServiceUpdate, update.Result, error)
	ReleaseKind() update.ReleaseKind
	ReleaseType() update.ReleaseType
	CommitMessage() string
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

	logger = log.NewContext(logger).With("type", "release")

	updates, results, err := changes.CalculateRelease(rc, logger)
	if err != nil {
		return nil, err
	}

	err = ApplyChanges(rc, updates, logger)
	return results, err
}

func ApplyChanges(rc *ReleaseContext, updates []*update.ServiceUpdate, logger log.Logger) error {
	logger.Log("updates", len(updates))
	if len(updates) == 0 {
		logger.Log("exit", "no images to update for services given")
		return nil
	}

	timer := update.NewStageTimer("push_changes")
	err := rc.WriteUpdates(updates)
	timer.ObserveDuration()
	return err
}
