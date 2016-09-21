package release

import (
	"sync"
	"time"

	"github.com/weaveworks/fluxy"
)

// InmemStore is an in-memory job store.
type InmemStore struct {
	mtx  sync.RWMutex
	jobs map[flux.ReleaseID]flux.ReleaseJob
}

// NewInmemStore returns a usable in-mem job store.
func NewInmemStore() *InmemStore {
	return &InmemStore{
		jobs: map[flux.ReleaseID]flux.ReleaseJob{},
	}
}

// GetJob implements JobStore.
func (s *InmemStore) GetJob(id flux.ReleaseID) (flux.ReleaseJob, error) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	j, ok := s.jobs[id]
	if !ok {
		return flux.ReleaseJob{}, flux.ErrNoSuchReleaseJob
	}
	return j, nil
}

// PutJob implements JobStore.
func (s *InmemStore) PutJob(spec flux.ReleaseJobSpec) (flux.ReleaseID, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	id := flux.NewReleaseID()
	for _, exists := s.jobs[id]; exists; id = flux.NewReleaseID() {
		// in case of ID collision
	}

	s.jobs[id] = flux.ReleaseJob{
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
	for _, j := range s.jobs {
		if j.Claimed.IsZero() && j.Submitted.Before(earliest) {
			candidate = j
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
func (s *InmemStore) UpdateJob(j flux.ReleaseJob) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.jobs[j.ID] = j
	return nil
}
