package jobs

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/release"
)

const (
	initialBackoff = 1 * time.Second
	maxBackoff     = 1 * time.Minute
)

var (
	ErrNoHandlerForJob = fmt.Errorf("no handler for job type")
)

type Handler interface {
	Handle(*flux.Job, flux.JobWritePopper) error
}

// Worker grabs jobs from the job store and executes them.
type Worker struct {
	jobs     flux.JobWritePopper
	handlers map[string]Handler
	logger   log.Logger
	stopping chan struct{}
	done     chan struct{}
}

// NewWorker returns a usable worker pulling jobs from the JobPopper.
// Run Work in its own goroutine to start execution.
func NewWorker(
	jobs flux.JobWritePopper,
	instancer instance.Instancer,
	metrics release.Metrics,
	logger log.Logger,
) *Worker {
	return &Worker{
		jobs: jobs,
		handlers: map[string]Handler{
			flux.ReleaseJob: newReleaseHandler(instancer, metrics),
		},
		logger:   logger,
		stopping: make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Work polls the job queue for new jobs.
// Call Stop() to stop the worker.
func (w *Worker) Work() {
	backoff := initialBackoff
	sleep := func() {
		time.Sleep(backoff)
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
	reset := func() {
		backoff = initialBackoff
	}
	for {
		select {
		case <-w.stopping:
			close(w.done)
			return
		default:
		}
		job, err := w.jobs.NextJob(nil)
		if err == flux.ErrNoJobAvailable {
			sleep()
			continue // normal
		}
		if err != nil {
			w.logger.Log("err", errors.Wrap(err, "fetch job")) // abnormal
			sleep()
			continue
		}
		reset()

		cancel, done := make(chan struct{}), make(chan struct{})
		go heartbeat(job.ID, w.jobs, time.Second, cancel, done, w.logger)

		job.Status = "Executing..."
		if err := w.jobs.UpdateJob(job); err != nil {
			w.logger.Log("err", errors.Wrapf(err, "updating job %s", job.ID))
		}

		if handler, ok := w.handlers[job.Method]; !ok {
			err = ErrNoHandlerForJob
		} else {
			err = handler.Handle(&job, w.jobs)
		}
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
			w.logger.Log("err", errors.Wrapf(err, "updating job %s", job.ID))
		}

		close(cancel)
		<-done
	}
}

// Close stops the worker from processing any more jobs
func (w *Worker) Stop(timeout time.Duration) error {
	close(w.stopping)
	select {
	case <-w.done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timout waiting for workers to shut down")
	}
}

func heartbeat(id flux.JobID, h heartbeater, d time.Duration, cancel <-chan struct{}, done chan<- struct{}, logger log.Logger) {
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
	Heartbeat(flux.JobID) error
}
