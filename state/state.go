package state

import (
	"sync"
)

// State repesents the running state of services, either cached from
// elsewhere or particular to fluxy
type State interface {
	ReleaseStateFor(namespace, service string) ReleaseState
}

// ReleaseUpdate is provided to a release process so it can give a
// running report of its progress
type ReleaseState interface {
	// Get the last reported release state
	Last() string
	// Update the state of the release to `msg`
	Update(msg string)
}

type namespacedService struct {
	namespace, service string
}

type serviceState struct {
	mtx           *sync.Mutex
	releaseStates map[namespacedService]string
}

func New() *serviceState {
	return &serviceState{
		mtx:           &sync.Mutex{},
		releaseStates: make(map[namespacedService]string),
	}
}

func (s *serviceState) ReleaseStateFor(namespace, service string) ReleaseState {
	return &releaseState{serviceState: s, service: namespacedService{namespace, service}}
}

type releaseState struct {
	*serviceState
	service namespacedService
}

func (s *releaseState) Update(msg string) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.releaseStates[s.service] = msg
}

func (s *releaseState) Last() string {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.releaseStates[s.service]
}
