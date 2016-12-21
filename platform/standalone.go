package platform

import (
	"errors"
	"sync"

	"github.com/weaveworks/flux"
)

type StandaloneMessageBus struct {
	connected map[flux.InstanceID]*removeablePlatform
	sync.RWMutex
	metrics BusMetrics
}

func NewStandaloneMessageBus(metrics BusMetrics) *StandaloneMessageBus {
	return &StandaloneMessageBus{
		connected: map[flux.InstanceID]*removeablePlatform{},
		metrics:   metrics,
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
// requests can be routed to it. Once the connection is closed --
// trying to use it is the only way to tell if it's closed -- the
// error representing the cause will be sent to the channel supplied.
func (s *StandaloneMessageBus) Subscribe(inst flux.InstanceID, p Platform, complete chan<- error) {
	s.Lock()
	// We're replacing another client
	if existing, ok := s.connected[inst]; ok {
		delete(s.connected, inst)
		s.metrics.IncrKicks(inst)
		existing.closeWithError(errors.New("duplicate connection; replacing with newer"))
	}

	done := make(chan error)
	s.connected[inst] = &removeablePlatform{
		remote: p,
		done:   done,
	}
	s.Unlock()

	// The only way we detect remote platforms closing are if an RPC
	// is attempted and it fails. When that happens, clean up behind
	// us.
	go func() {
		err := <-done
		s.Lock()
		if existing, ok := s.connected[inst]; ok && existing.remote == p {
			delete(s.connected, inst)
		}
		s.Unlock()
		complete <- err
	}()
}

// Ping returns nil if the specified instance is connected, and
// ErrPlatformNotAvailable if not.
func (s *StandaloneMessageBus) Ping(inst flux.InstanceID) error {
	var (
		p  Platform
		ok bool
	)
	s.RLock()
	p, ok = s.connected[inst]
	s.RUnlock()

	if ok {
		return p.Ping()
	}
	return ErrPlatformNotAvailable
}

type removeablePlatform struct {
	remote Platform
	done   chan error
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
		if _, ok := err.(FatalError); ok {
			p.closeWithError(err)
		}
	}()
	return p.remote.AllServices(maybeNamespace, ignored)
}

func (p *removeablePlatform) SomeServices(ids []flux.ServiceID) (s []Service, err error) {
	defer func() {
		if _, ok := err.(FatalError); ok {
			p.closeWithError(err)
		}
	}()
	return p.remote.SomeServices(ids)
}

func (p *removeablePlatform) Apply(defs []ServiceDefinition) (err error) {
	defer func() {
		if _, ok := err.(FatalError); ok {
			p.closeWithError(err)
		}
	}()
	return p.remote.Apply(defs)
}

func (p *removeablePlatform) Ping() (err error) {
	defer func() {
		if _, ok := err.(FatalError); ok {
			p.closeWithError(err)
		}
	}()
	return p.remote.Ping()
}

type disconnectedPlatform struct{}

func (p disconnectedPlatform) AllServices(string, flux.ServiceIDSet) ([]Service, error) {
	return nil, ErrPlatformNotAvailable
}

func (p disconnectedPlatform) SomeServices([]flux.ServiceID) ([]Service, error) {
	return nil, ErrPlatformNotAvailable
}

func (p disconnectedPlatform) Apply([]ServiceDefinition) error {
	return ErrPlatformNotAvailable
}

func (p disconnectedPlatform) Ping() error {
	return ErrPlatformNotAvailable
}
