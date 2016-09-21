package release

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/weaveworks/fluxy"
)

// JobStore collects behaviors necessary for interacting with jobs.
type JobStore interface {
	JobReadWriter
	JobPopper
}

// JobReadWriter collects behaviors necessary for reading and writing jobs.
type JobReadWriter interface {
	GetJob(ID) (Job, error)
	PutJob(JobSpec) (ID, error)
}

// JobPopper collects behaviors necessary for getting and executing jobs.
type JobPopper interface {
	NextJob() (Job, error)
	UpdateJob(Job) error
}

var (
	// ErrNoSuchJob is returned when a job ID is not found in the store.
	ErrNoSuchJob = errors.New("no such job found")

	// ErrNoJobAvailable is returned by NextJob when there's no pending job.
	ErrNoJobAvailable = errors.New("no job available")
)

// ID is a release ID, a UUID.
type ID string

func newID() ID {
	b := make([]byte, 16)
	rand.Read(b)
	return ID(fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]))
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Job collects a job spec, an ID, and details about the progress.
type Job struct {
	Spec      JobSpec   `json:"spec"`
	ID        ID        `json:"id"`
	Submitted time.Time `json:"submitted"`
	Claimed   time.Time `json:"claimed,omitempty"`
	Started   time.Time `json:"started,omitempty"`
	Status    string    `json:"status"`
	Log       []string  `json:"log,omitempty"`
	Finished  time.Time `json:"finished,omitempty"`
	Success   bool      `json:"success"` // only makes sense after Finished
}

// JobSpec is the things that a user requests when making a release.
type JobSpec struct {
	ServiceSpec flux.ServiceSpec
	ImageSpec   flux.ImageSpec
	Kind        flux.ReleaseKind
}

// InmemStore is an in-memory job store.
type InmemStore struct {
	mtx  sync.RWMutex
	jobs map[ID]Job
}

// NewInmemStore returns a usable in-mem job store.
func NewInmemStore() *InmemStore {
	return &InmemStore{
		jobs: map[ID]Job{},
	}
}

// GetJob implements JobStore.
func (s *InmemStore) GetJob(id ID) (Job, error) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	j, ok := s.jobs[id]
	if !ok {
		return Job{}, ErrNoSuchJob
	}
	return j, nil
}

// PutJob implements JobStore.
func (s *InmemStore) PutJob(spec JobSpec) (ID, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	id := newID()
	for _, exists := s.jobs[id]; exists; id = newID() {
		// in case of ID collision
	}

	s.jobs[id] = Job{
		Spec:      spec,
		ID:        id,
		Submitted: time.Now(),
	}

	return id, nil
}

// NextJob implements JobStore.
// It returns immediately. If no job is available, ErrNoJobAvailable is returned.
func (s *InmemStore) NextJob() (Job, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	var (
		candidate Job
		earliest  = time.Now()
	)
	for _, j := range s.jobs {
		if j.Claimed.IsZero() && j.Submitted.Before(earliest) {
			candidate = j
		}
	}

	if candidate.ID == "" {
		return Job{}, ErrNoJobAvailable
	}

	candidate.Claimed = time.Now()
	s.jobs[candidate.ID] = candidate
	return candidate, nil
}

// UpdateJob implements JobStore.
func (s *InmemStore) UpdateJob(j Job) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.jobs[j.ID] = j
	return nil
}
