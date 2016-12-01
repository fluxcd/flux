package release

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
)

const backoff = 1 * time.Second

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
	metrics Metrics,
	logger log.Logger,
) *Worker {
	return &Worker{
		jobs:     jobs,
		releaser: newReleaser(instancer, metrics),
		logger:   logger,
	}
}

// Work takes and executes a job every time the tick chan fires.
// Create a time.NewTicker() and pass ticker.C as the tick chan.
// Stop the ticker to stop the worker.
func (w *Worker) Work(shutdown <-chan struct{}) {
	for {
		select {
		case <-shutdown:
			return
		default:
			break
		}

		job, err := w.jobs.NextJob()
		if err == flux.ErrNoReleaseJobAvailable {
			time.Sleep(backoff)
			continue
		}
		if err != nil {
			w.logger.Log("err", errors.Wrap(err, "fetch release job")) // abnormal
			continue
		}

		cancel, done := make(chan struct{}), make(chan struct{})
		go heartbeat(job.ID, w.jobs, time.Second, cancel, done, w.logger)

		job.Status = "Executing..."
		if err := w.jobs.UpdateJob(job); err != nil {
			w.logger.Log("err", errors.Wrapf(err, "updating release job %s", job.ID))
		}

		err = w.releaser.Release(&job, w.jobs)
		job.Done = true
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

		close(cancel)
		<-done
	}
}

func heartbeat(id flux.ReleaseID, h heartbeater, d time.Duration, cancel <-chan struct{}, done chan<- struct{}, logger log.Logger) {
	t := time.NewTicker(d)
	defer t.Stop()
	defer close(done)
	for {
		select {
		case <-t.C:
			if err := h.Heartbeat(id); err != nil {
				logger.Log("heartbeat", err)
			}
		case <-cancel:
			return
		}
	}
}

type heartbeater interface {
	Heartbeat(flux.ReleaseID) error
}
