package release

import (
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/pkg/errors"

	"github.com/weaveworks/fluxy"
	"github.com/weaveworks/fluxy/git"
	"github.com/weaveworks/fluxy/history"
	"github.com/weaveworks/fluxy/platform/kubernetes"
	"github.com/weaveworks/fluxy/registry"
)

// Worker grabs release jobs from the job store and executes them.
type Worker struct {
	popper   flux.ReleaseJobPopper
	releaser *releaser
	logger   log.Logger
}

// NewWorker returns a usable worker pulling jobs from the JobPopper.
// Run Work in its own goroutine to start execution.
func NewWorker(
	popper flux.ReleaseJobPopper,
	platform *kubernetes.Cluster,
	registry *registry.Client,
	repo git.Repo,
	history history.EventWriter,
	metrics Metrics,
	helperDuration metrics.Histogram,
	logger log.Logger,
) *Worker {
	return &Worker{
		popper:   popper,
		releaser: newReleaser(platform, registry, logger, repo, history, metrics, helperDuration),
		logger:   logger,
	}
}

// Work takes and executes a job every time the tick chan fires.
// Create a time.NewTicker() and pass ticker.C as the tick chan.
// Stop the ticker to stop the worker.
func (w *Worker) Work(tick <-chan time.Time) {
	for range tick {
		j, err := w.popper.NextJob()
		if err == flux.ErrNoReleaseJobAvailable {
			continue // normal
		}
		if err != nil {
			w.logger.Log("err", errors.Wrap(err, "fetch release job")) // abnormal
			continue
		}

		j.Started = time.Now()
		j.Status = "Executing..."
		if err := w.popper.UpdateJob(j); err != nil {
			w.logger.Log("err", errors.Wrapf(err, "updating release job %s", j.ID))
		}

		// TODO(pb): update Release to take a Job and continuously Update it
		// instead of returning release actions.
		res, err := w.releaser.Release(j.Spec.ServiceSpec, j.Spec.ImageSpec, j.Spec.Kind)
		j.Finished = time.Now()
		if err != nil {
			j.Success = false
			j.Status = err.Error()
		} else {
			j.Success = true
			j.Status = "Complete."
			j.Log = actions2log(res)
			j.TemporaryReleaseActions = res
		}
		if err := w.popper.UpdateJob(j); err != nil {
			w.logger.Log("err", errors.Wrapf(err, "updating release job %s", j.ID))
		}
	}
}

func actions2log(res []flux.ReleaseAction) []string {
	a := make([]string, len(res))
	for i := range res {
		a[i] = res[i].Description
	}
	return a
}
