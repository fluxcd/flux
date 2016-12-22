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

	// AutomatedServiceJob is the method for a check automated service job
	AutomatedServiceJob = "automated_service"

	// AutomatedInstanceJob is the method for a check automated instance job
	AutomatedInstanceJob = "automated_instance"

	// PriorityBackground is priority for background jobs
	PriorityBackground = 100

	// PriorityInteractive is priority for interactive jobs
	PriorityInteractive = 200
)

var (
	ErrNoSuchJob        = errors.New("no such release job found")
	ErrNoJobAvailable   = errors.New("no job available")
	ErrUnknownJobMethod = errors.New("unknown job method")
	ErrJobAlreadyQueued = errors.New("job is already queued")
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
	Submitted time.Time `json:"submitted"`
	Claimed   time.Time `json:"claimed,omitempty"`
	Heartbeat time.Time `json:"heartbeat,omitempty"`
	Finished  time.Time `json:"finished,omitempty"`
	Log       []string  `json:"log,omitempty"`
	Status    string    `json:"status"`
	Done      bool      `json:"done"`
	Success   bool      `json:"success"` // only makes sense after done is true
}

func (j *Job) UnmarshalJSON(data []byte) error {
	var wireJob struct {
		Instance flux.InstanceID `json:"instanceID"`
		ID       JobID           `json:"id"`

		// To be set when scheduling the job
		Queue       string          `json:"queue"`
		Method      string          `json:"method"`
		Params      json.RawMessage `json:"params"`
		ScheduledAt time.Time       `json:"scheduled_at"`
		Priority    int             `json:"priority"`

		// Key is an optional field, and can be used to create jobs iff a pending
		// job with the same key doesn't exist.
		Key string `json:"key,omitempty"`

		// To be used by the worker
		Submitted time.Time `json:"submitted"`
		Claimed   time.Time `json:"claimed,omitempty"`
		Heartbeat time.Time `json:"heartbeat,omitempty"`
		Finished  time.Time `json:"finished,omitempty"`
		Log       []string  `json:"log,omitempty"`
		Status    string    `json:"status"`
		Done      bool      `json:"done"`
		Success   bool      `json:"success"` // only makes sense after done is true
	}
	if err := json.Unmarshal(data, &wireJob); err != nil {
		return err
	}
	*j = Job{
		Instance:    wireJob.Instance,
		ID:          wireJob.ID,
		Queue:       wireJob.Queue,
		Method:      wireJob.Method,
		ScheduledAt: wireJob.ScheduledAt,
		Priority:    wireJob.Priority,
		Key:         wireJob.Key,
		Submitted:   wireJob.Submitted,
		Claimed:     wireJob.Claimed,
		Heartbeat:   wireJob.Heartbeat,
		Finished:    wireJob.Finished,
		Log:         wireJob.Log,
		Status:      wireJob.Status,
		Done:        wireJob.Done,
		Success:     wireJob.Success,
	}
	switch j.Method {
	case ReleaseJob:
		var p ReleaseJobParams
		if err := json.Unmarshal(wireJob.Params, &p); err != nil {
			return err
		}
		j.Params = p
	}
	return nil
}

// ReleaseJobParams are the params for a release job
type ReleaseJobParams struct {
	ServiceSpec  flux.ServiceSpec
	ServiceSpecs []flux.ServiceSpec
	ImageSpec    flux.ImageSpec
	Kind         flux.ReleaseKind
	Excludes     []flux.ServiceID
}

// AutomatedServiceJobParams are the params for a automated_service job
type AutomatedServiceJobParams struct {
	ServiceSpec flux.ServiceSpec
}

// AutomatedInstanceJobParams are the params for an automated_instance job
type AutomatedInstanceJobParams struct {
	InstanceID flux.InstanceID
}
