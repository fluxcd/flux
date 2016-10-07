package release

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/fluxy"
	"github.com/weaveworks/fluxy/git"
	"github.com/weaveworks/fluxy/instance"
)

// Worker grabs release jobs from the job store and executes them.
type Worker struct {
	jobs     flux.ReleaseJobWritePopper
	releaser *releaser
	logger   log.Logger
}

// NewWorker returns a usable worker pulling jobs from the JobPopper.
// Run Work in its own goroutine to start execution.
func NewWorker(
	jobs flux.ReleaseJobWritePopper,
	instancer instance.Instancer,
	repo git.Repo,
	metrics Metrics,
	logger log.Logger,
) *Worker {
	return &Worker{
		jobs:     jobs,
		releaser: newReleaser(instancer, repo, metrics),
		logger:   logger,
	}
}

// Work takes and executes a job every time the tick chan fires.
// Create a time.NewTicker() and pass ticker.C as the tick chan.
// Stop the ticker to stop the worker.
func (w *Worker) Work(tick <-chan time.Time) {
	for range tick {
		job, err := w.jobs.NextJob()
		if err == flux.ErrNoReleaseJobAvailable {
			continue // normal
		}
		if err != nil {
			w.logger.Log("err", errors.Wrap(err, "fetch release job")) // abnormal
			continue
		}

		job.Started = time.Now()
		job.Status = "Executing..."
		if err := w.jobs.UpdateJob(job); err != nil {
			w.logger.Log("err", errors.Wrapf(err, "updating release job %s", job.ID))
		}

		err = w.releaser.Release(&job, w.jobs)
		job.Finished = time.Now()
		if err != nil {
			job.Success = false
			status := fmt.Sprintf("Failed: %v", err)
			job.Status = status
			job.Log = append(job.Log, status)
		} else {
			job.Success = true
			job.Status = "Complete."
		}
		if err := w.jobs.UpdateJob(job); err != nil {
			w.logger.Log("err", errors.Wrapf(err, "updating release job %s", job.ID))
		}
	}
}
