package flux

import (
	"errors"
	"fmt"
	"math/rand"
	"time"
)

const (
	// DefaultQueue is the queue to use if none is set.
	DefaultQueue = "default"

	// ReleaseJob is the method for a release job
	ReleaseJob = "release"

	// PriorityBackground is priority for background jobs
	PriorityBackground = 100

	// PriorityInteractive is priority for interactive jobs
	PriorityInteractive = 200
)

var (
	ErrNoSuchJob        = errors.New("no such release job found")
	ErrNoJobAvailable   = errors.New("no job available")
	ErrUnknownJobMethod = errors.New("unknown job method")
)

type JobStore interface {
	JobReadPusher
	JobWritePopper
	GC() error
}

type JobReadPusher interface {
	GetJob(InstanceID, JobID) (Job, error)
	PutJob(InstanceID, Job) (JobID, error)
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
	b := make([]byte, 16)
	rand.Read(b)
	return JobID(fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]))
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Job describes a worker job
type Job struct {
	Instance InstanceID `json:"instanceID"`
	ID       JobID      `json:"id"`

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

// ReleaseJobParams are the params for a release job
type ReleaseJobParams struct {
	ServiceSpec ServiceSpec
	ImageSpec   ImageSpec
	Kind        ReleaseKind
	Excludes    []ServiceID
}
