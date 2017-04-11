package jobs

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/guid"
)

const (
	// DefaultQueue is the queue to use if none is set.
	DefaultQueue = "default"

	// ReleaseJob is the method for a release job
	ReleaseJob = "release"

	// AutomatedInstanceJob is the method for a check automated instance job
	AutomatedInstanceJob = "automated_instance"

	// SyncJob is the method for a sync job
	SyncJob = "sync"

	// PriorityBackground is priority for background jobs
	PriorityBackground = 100

	// PriorityInteractive is priority for interactive jobs
	PriorityInteractive = 200
)

var (
	// This is a user-facing error
	ErrNoSuchJob = flux.Missing{&flux.BaseError{
		Help: `The release you requested does not exist.

This may mean that it has expired, or that you have mistyped the
release ID.`,
		Err: errors.New("no such release job found"),
	}}

	ErrNoJobAvailable   = errors.New("no job available")
	ErrUnknownJobMethod = errors.New("unknown job method")
	ErrJobAlreadyQueued = errors.New("job is already queued")
	ErrNoResultExpected = errors.New("no result expected")
)

type JobStore interface {
	JobReadPusher
	JobWritePopper
	GC() error
}

type JobReadPusher interface {
	GetJob(flux.InstanceID, JobID) (Job, error)
	PutJob(flux.InstanceID, Job) (JobID, error)
	PutJobIgnoringDuplicates(flux.InstanceID, Job) (JobID, error)
}

type JobWritePopper interface {
	JobUpdater
	JobPopper
}

type JobUpdater interface {
	UpdateJob(Job) error
	Heartbeat(JobID) error
}

type JobPopper interface {
	NextJob(queues []string) (Job, error)
}

type JobID string

func NewJobID() JobID {
	return JobID(guid.New())
}

// Job describes a worker job
type Job struct {
	Instance flux.InstanceID `json:"instanceID"`
	ID       JobID           `json:"id"`

	// To be set when scheduling the job
	Queue       string      `json:"queue"`
	Method      string      `json:"method"`
	Params      interface{} `json:"params"`
	ScheduledAt time.Time   `json:"scheduled_at"`
	Priority    int         `json:"priority"`

	// Key is an optional field, and can be used to create jobs iff a pending
	// job with the same key doesn't exist.
	Key string `json:"key,omitempty"`

	// To be used by the worker
	Submitted time.Time       `json:"submitted"`
	Claimed   time.Time       `json:"claimed,omitempty"`
	Heartbeat time.Time       `json:"heartbeat,omitempty"`
	Finished  time.Time       `json:"finished,omitempty"`
	Log       []string        `json:"log,omitempty"`
	Result    interface{}     `json:"result"` // may be updated to reflect progress
	Status    string          `json:"status"`
	Done      bool            `json:"done"`
	Success   bool            `json:"success"` // only makes sense after done is true
	Error     *flux.BaseError `json:"error,omitempty"`
}

func (j *Job) UnmarshalJSON(data []byte) error {
	type JobAlias Job
	var wireJob struct {
		*JobAlias
		Params json.RawMessage `json:"params"`
		Result json.RawMessage `json:"result"`
	}
	wireJob.JobAlias = (*JobAlias)(j)
	if err := json.Unmarshal(data, &wireJob); err != nil {
		return err
	}

	switch j.Method {
	case ReleaseJob:
		var p ReleaseJobParams
		if wireJob.Params != nil {
			if err := json.Unmarshal(wireJob.Params, &p); err != nil {
				return err
			}
		}
		j.Params = p
		var r flux.ReleaseResult
		if wireJob.Result != nil {
			if err := json.Unmarshal(wireJob.Result, &r); err != nil {
				return err
			}
		}
		j.Result = r
	}
	return nil
}

// ReleaseJobParams are the params for a release job
type ReleaseJobParams struct {
	flux.ReleaseSpec
	Cause flux.ReleaseCause
}

func (params ReleaseJobParams) Spec() flux.ReleaseSpec {
	return params.ReleaseSpec
}

// AutomatedInstanceJobParams are the params for an automated_instance job
type AutomatedInstanceJobParams struct {
	InstanceID flux.InstanceID
}

// SyncJobParams are the params for a sync job
type SyncJobParams struct {
	InstanceID flux.InstanceID
}
