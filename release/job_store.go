package release

import (
	"sync"
	"time"

	"github.com/weaveworks/fluxy"
)

// InmemStore is an in-memory job store.
type InmemStore struct {
	mtx    sync.RWMutex
	jobs   map[flux.ReleaseID]flux.ReleaseJob
	oldest time.Duration
}

// NewInmemStore returns a usable in-mem job store.
func NewInmemStore(oldest time.Duration) *InmemStore {
	return &InmemStore{
		jobs:   map[flux.ReleaseID]flux.ReleaseJob{},
		oldest: oldest,
	}
}

// GetJob implements JobStore.
func (s *InmemStore) GetJob(inst flux.InstanceID, id flux.ReleaseID) (flux.ReleaseJob, error) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	job, ok := s.jobs[id]
	if !ok {
		return flux.ReleaseJob{}, flux.ErrNoSuchReleaseJob
	}
	return job, nil
}

// PutJob implements JobStore.
func (s *InmemStore) PutJob(inst flux.InstanceID, spec flux.ReleaseJobSpec) (flux.ReleaseID, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	id := flux.NewReleaseID()
	for _, exists := s.jobs[id]; exists; id = flux.NewReleaseID() {
		// in case of ID collision
	}

	s.jobs[id] = flux.ReleaseJob{
		Instance:  inst,
		Spec:      spec,
		ID:        id,
		Submitted: time.Now(),
	}

	return id, nil
}

// NextJob implements JobStore.
// It returns immediately. If no job is available, ErrNoJobAvailable is returned.
func (s *InmemStore) NextJob() (flux.ReleaseJob, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	var (
		candidate flux.ReleaseJob
		earliest  = time.Now()
	)
	for _, job := range s.jobs {
		if job.Claimed.IsZero() && job.Submitted.Before(earliest) {
			candidate = job
		}
	}

	if candidate.ID == "" {
		return flux.ReleaseJob{}, flux.ErrNoReleaseJobAvailable
	}

	candidate.Claimed = time.Now()
	s.jobs[candidate.ID] = candidate
	return candidate, nil
}

// UpdateJob implements JobStore.
func (s *InmemStore) UpdateJob(job flux.ReleaseJob) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.jobs[job.ID] = job
	return nil
}

func (s *InmemStore) GC() error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	cutoff := time.Now().Add(-s.oldest)
	for id, job := range s.jobs {
		if job.IsFinished() && job.Finished.Before(cutoff) {
			delete(s.jobs, id)
		}
	}
	return nil
}
