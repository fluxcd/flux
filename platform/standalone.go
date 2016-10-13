package platform

import (
	"sync"

	"github.com/weaveworks/fluxy"
)

type StandaloneMessageBus struct {
	connected map[flux.InstanceID]*removeablePlatform
	sync.RWMutex
}

func NewStandaloneMessageBus() *StandaloneMessageBus {
	return &StandaloneMessageBus{
		connected: map[flux.InstanceID]*removeablePlatform{},
	}
}

func (s *StandaloneMessageBus) Connect(inst flux.InstanceID) (Platform, error) {
	s.RLock()
	defer s.RUnlock()
	p, ok := s.connected[inst]
	if !ok {
		return nil, ErrPlatformNotAvailable
	}
	return p, nil
}

func (s *StandaloneMessageBus) Subscribe(inst flux.InstanceID, p Platform) error {
	s.Lock()
	// We're replacing another client
	if existing, ok := s.connected[inst]; ok {
		delete(s.connected, inst)
		existing.handleError((*error)(nil))
	}

	// Add our new client in
	done := make(chan error)
	s.connected[inst] = &removeablePlatform{
		Platform: p,
		done:     done,
	}
	s.Unlock()

	// Wait to be kicked, or an error to happen
	err := <-done

	// Cleanup behind us, in case we're not being kicked.
	s.Lock()
	if existing, ok := s.connected[inst]; ok && existing == p {
		delete(s.connected, inst)
	}
	s.Unlock()
	return err
}

type removeablePlatform struct {
	Platform
	done chan error
	sync.Mutex
}

func (p *removeablePlatform) handleError(err *error) {
	p.Lock()
	if p.done != nil {
		p.done <- *err
		close(p.done)
		p.done = nil
	}
	p.Unlock()
}

func (p *removeablePlatform) AllServices(maybeNamespace string, ignored flux.ServiceIDSet) (s []Service, err error) {
	defer p.handleError(&err)
	return p.Platform.AllServices(maybeNamespace, ignored)
}

func (p *removeablePlatform) SomeServices(ids []flux.ServiceID) (s []Service, err error) {
	defer p.handleError(&err)
	return p.Platform.SomeServices(ids)
}

func (p *removeablePlatform) Regrade(spec []RegradeSpec) (err error) {
	defer p.handleError(&err)
	return p.Platform.Regrade(spec)
}
