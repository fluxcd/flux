package jobs

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	fluxmetrics "github.com/weaveworks/flux/metrics"
)

const (
	pollingPeriod = 1 * time.Second
)

var (
	ErrNoHandlerForJob = fmt.Errorf("no handler for job type")
)

type Handler interface {
	Handle(*Job, JobUpdater) ([]Job, error)
}

// Worker grabs jobs from the job store and executes them.
type Worker struct {
	jobs     JobStore
	handlers map[string]Handler
	metrics  WorkerMetrics
	logger   log.Logger
	queues   []string
	stopping chan struct{}
	done     chan struct{}
}

// NewWorker returns a usable worker pulling jobs from the JobPopper.
// Run Work in its own goroutine to start execution.
func NewWorker(
	jobs JobStore,
	logger log.Logger,
	metrics WorkerMetrics,
	queues []string,
) *Worker {
	return &Worker{
		jobs:     jobs,
		handlers: map[string]Handler{},
		metrics:  metrics,
		logger:   logger,
		queues:   queues,
		stopping: make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Register registers a new handler for a method
func (w *Worker) Register(jobMethod string, handler Handler) {
	w.handlers[jobMethod] = handler
}

// Work polls the job queue for new jobs.
// Call Stop() to stop the worker.
func (w *Worker) Work() {
	for {
		select {
		case <-w.stopping:
			close(w.done)
			return
		default:
		}
		job, err := w.jobs.NextJob(w.queues)
		if err == ErrNoJobAvailable {
			time.Sleep(pollingPeriod)
			continue // normal
		}
		if err != nil {
			w.logger.Log("err", errors.Wrap(err, "fetch job")) // abnormal
			time.Sleep(pollingPeriod)
			continue
		}
		logger := log.NewContext(w.logger).With("job", job.ID)
		logger.Log("method", job.Method)

		cancel, done := make(chan struct{}), make(chan struct{})
		go heartbeat(job.ID, w.jobs, time.Second, cancel, done, logger)

		job.Status = "Executing..."
		if err := w.jobs.UpdateJob(job); err != nil {
			logger.Log("err", errors.Wrap(err, "updating job"))
		}

		begin := time.Now().UTC()
		var followUps []Job
		if handler, ok := w.handlers[job.Method]; !ok {
			err = ErrNoHandlerForJob
		} else {
			followUps, err = handler.Handle(&job, w.jobs)
		}
		w.metrics.JobDuration.With(
			fluxmetrics.LabelMethod, job.Method,
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
		logger.Log("took", time.Since(begin))
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
			logger.Log("err", errors.Wrap(err, "updating job"))
		}

		// Schedule any follow-up jobs
		for _, followUp := range followUps {
			if _, err := w.jobs.PutJob(job.Instance, followUp); err != nil && err != ErrJobAlreadyQueued {
				logger.Log("err", errors.Wrap(err, "putting follow-up job"))
			}
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

func heartbeat(id JobID, h heartbeater, d time.Duration, cancel <-chan struct{}, done chan<- struct{}, logger log.Logger) {
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
	Heartbeat(JobID) error
}
