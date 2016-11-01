package platform

import (
	"errors"
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

// Connect hands back a platform, given an instance ID. Since the
// platform will not always be connected, and we want to be able to
// process operations that don't involve a platform (like setting
// config), we have a special value for a disconnected platform,
// rather than returning an error.
func (s *StandaloneMessageBus) Connect(inst flux.InstanceID) (Platform, error) {
	s.RLock()
	defer s.RUnlock()
	p, ok := s.connected[inst]
	if !ok {
		return disconnectedPlatform{}, nil
	}
	return p, nil
}

// Subscribe introduces a Platform to the message bus, so that
// requests can be routed to it. This will block until the connection
// is closed, or replaced by another.
func (s *StandaloneMessageBus) Subscribe(inst flux.InstanceID, p Platform) error {
	s.Lock()
	// We're replacing another client
	if existing, ok := s.connected[inst]; ok {
		delete(s.connected, inst)
		existing.closeWithError(errors.New("duplicate connection; replacing with newer"))
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

func (p *removeablePlatform) closeWithError(err error) {
	p.Lock()
	defer p.Unlock()
	if p.done != nil {
		p.done <- err
		close(p.done)
		p.done = nil
	}
}

func (p *removeablePlatform) AllServices(maybeNamespace string, ignored flux.ServiceIDSet) (s []Service, err error) {
	defer func() {
		if err != nil {
			p.closeWithError(err)
		}
	}()
	return p.Platform.AllServices(maybeNamespace, ignored)
}

func (p *removeablePlatform) SomeServices(ids []flux.ServiceID) (s []Service, err error) {
	defer func() {
		if err != nil {
			p.closeWithError(err)
		}
	}()
	return p.Platform.SomeServices(ids)
}

func (p *removeablePlatform) Regrade(spec []RegradeSpec) (err error) {
	defer func() {
		if err != nil {
			p.closeWithError(err)
		}
	}()
	return p.Platform.Regrade(spec)
}

type disconnectedPlatform struct{}

func (p disconnectedPlatform) AllServices(string, flux.ServiceIDSet) ([]Service, error) {
	return nil, ErrPlatformNotAvailable
}

func (p disconnectedPlatform) SomeServices([]flux.ServiceID) ([]Service, error) {
	return nil, ErrPlatformNotAvailable
}

func (p disconnectedPlatform) Regrade([]RegradeSpec) error {
	return ErrPlatformNotAvailable
}
