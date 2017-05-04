package remote

import (
	"errors"
	"sync"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/update"
)

var (
	errNotSubscribed = UnavailableError(errors.New("daemon not subscribed"))
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
// an error if not.
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
	return errNotSubscribed
}

// Version returns the fluxd version for the connected instance if the
// specified instance is connected, and an error if not.
func (s *StandaloneMessageBus) Version(inst flux.InstanceID) (string, error) {
	var (
		p  Platform
		ok bool
	)
	s.RLock()
	p, ok = s.connected[inst]
	s.RUnlock()

	if ok {
		return p.Version()
	}
	return "", errNotSubscribed
}

type removeablePlatform struct {
	remote Platform
	done   chan error
	sync.Mutex
}

func (p *removeablePlatform) closeWithError(err error) {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()
	if p.done != nil {
		p.done <- err
		close(p.done)
		p.done = nil
	}
}

func (p *removeablePlatform) Ping() (err error) {
	defer func() {
		if _, ok := err.(FatalError); ok {
			p.closeWithError(err)
		}
	}()
	return p.remote.Ping()
}

func (p *removeablePlatform) Version() (v string, err error) {
	defer func() {
		if _, ok := err.(FatalError); ok {
			p.closeWithError(err)
		}
	}()
	return p.remote.Version()
}

func (p *removeablePlatform) Export() (config []byte, err error) {
	defer func() {
		if _, ok := err.(FatalError); ok {
			p.closeWithError(err)
		}
	}()
	return p.remote.Export()
}

func (p *removeablePlatform) ListServices(namespace string) (_ []flux.ServiceStatus, err error) {
	defer func() {
		if _, ok := err.(FatalError); ok {
			p.closeWithError(err)
		}
	}()
	return p.remote.ListServices(namespace)
}

func (p *removeablePlatform) ListImages(spec update.ServiceSpec) (_ []flux.ImageStatus, err error) {
	defer func() {
		if _, ok := err.(FatalError); ok {
			p.closeWithError(err)
		}
	}()
	return p.remote.ListImages(spec)
}

func (p *removeablePlatform) UpdateManifests(u update.Spec) (_ job.ID, err error) {
	defer func() {
		if _, ok := err.(FatalError); ok {
			p.closeWithError(err)
		}
	}()
	return p.remote.UpdateManifests(u)
}

func (p *removeablePlatform) SyncNotify() (err error) {
	defer func() {
		if _, ok := err.(FatalError); ok {
			p.closeWithError(err)
		}
	}()
	return p.remote.SyncNotify()
}

func (p *removeablePlatform) JobStatus(id job.ID) (_ job.Status, err error) {
	defer func() {
		if _, ok := err.(FatalError); ok {
			p.closeWithError(err)
		}
	}()
	return p.remote.JobStatus(id)
}

func (p *removeablePlatform) SyncStatus(ref string) (revs []string, err error) {
	defer func() {
		if _, ok := err.(FatalError); ok {
			p.closeWithError(err)
		}
	}()
	return p.remote.SyncStatus(ref)
}

// disconnectedPlatform is a stub implementation used when the
// platform is known to be missing.

type disconnectedPlatform struct{}

func (p disconnectedPlatform) Ping() error {
	return errNotSubscribed
}

func (p disconnectedPlatform) Version() (string, error) {
	return "", errNotSubscribed
}

func (p disconnectedPlatform) Export() ([]byte, error) {
	return nil, errNotSubscribed
}

func (p disconnectedPlatform) ListServices(namespace string) ([]flux.ServiceStatus, error) {
	return nil, errNotSubscribed
}

func (p disconnectedPlatform) ListImages(update.ServiceSpec) ([]flux.ImageStatus, error) {
	return nil, errNotSubscribed
}

func (p disconnectedPlatform) UpdateManifests(update.Spec) (job.ID, error) {
	var id job.ID
	return id, errNotSubscribed
}

func (p disconnectedPlatform) SyncNotify() error {
	return errNotSubscribed
}

func (p disconnectedPlatform) JobStatus(id job.ID) (job.Status, error) {
	return job.Status{}, errNotSubscribed
}

func (p disconnectedPlatform) SyncStatus(string) ([]string, error) {
	return nil, errNotSubscribed
}
