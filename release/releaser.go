package release

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux/update"
)

type Changes interface {
	CalculateRelease(update.ReleaseContext, log.Logger) ([]*update.ServiceUpdate, update.Result, error)
	ReleaseKind() update.ReleaseKind
	CommitMessage() string
}

type Observer interface {
	Observe(time.Time, error)
}

func Release(rc *ReleaseContext, changes Changes, logger log.Logger) (results update.Result, err error) {
	if o, ok := changes.(Observer); ok {
		defer func(start time.Time) {
			o.Observe(start, err)
		}(time.Now())
	}

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
	} else {
		l := log.NewContext(logger).With("msg", "applying changes")
		for _, u := range updates {
			l.Log("changes", fmt.Sprintf("%#v", *u))
		}
	}

	timer := update.NewStageTimer("push_changes")
	err := rc.WriteUpdates(updates)
	timer.ObserveDuration()
	return err
}
