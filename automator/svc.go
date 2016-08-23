package automator

import (
	"fmt"
	"time"

	"github.com/weaveworks/fluxy"
	"github.com/weaveworks/fluxy/history"
)

type state string

const (
	stateInvalid   state = "Invalid"
	stateDeleting        = "Deleting"
	stateReleasing       = "Releasing"
	stateWaiting         = "Waiting"
)

type serviceLogFunc func(format string, args ...interface{})

func makeServiceLogFunc(his history.DB, namespace, serviceName string) serviceLogFunc {
	return func(format string, args ...interface{}) {
		his.LogEvent(namespace, serviceName, "Automation: "+fmt.Sprintf(format, args...))
	}
}

var (
	waitingTimeDefault = time.Minute
	waitingTimeFast    = 15 * time.Second
)

// svc is the atomic unit which is managed by the automator.
// It is one-to-one with a platform service.
type svc struct {
	service  flux.ServiceID
	st       state
	logf     serviceLogFunc
	waitc    <-chan time.Time // optionally set before moving to Waiting
	deletec  chan struct{}    // close to move to Deleting
	onDelete func()
	cfg      Config
}

func newSvc(namespace, serviceName string, logf serviceLogFunc, onDelete func(), cfg Config) *svc {
	id := flux.MakeServiceID(namespace, serviceName)
	s := &svc{
		service:  id,
		st:       stateWaiting,
		logf:     logf,
		waitc:    nil,
		deletec:  make(chan struct{}),
		onDelete: onDelete,
		cfg:      cfg,
	}
	go s.loop()
	return s
}

// See doc.go for an illustration of the service state machine.
func (s *svc) loop() {
	s.logf("service activated")
	defer s.logf("service deactivated")

	for {
		switch s.st {
		case stateDeleting:
			s.onDelete()
			return

		case stateReleasing:
			s.st = s.releasing(s.cfg)

		case stateWaiting:
			s.st = s.waiting()
		}
	}
}

func (s *svc) signalDelete() {
	s.logf("entering Deleting state")
	close(s.deletec)
}

func (s *svc) releasing(cfg Config) state {
	s.logf("entering Releasing state")

	if _, err := s.cfg.Releaser.Release(
		flux.ServiceSpec(s.service),
		flux.ImageSpecLatest,
		flux.ReleaseKindExecute,
	); err != nil {
		s.logf("%v", err)
	}
	return stateWaiting
}

func (s *svc) waiting() state {
	s.logf("entering Waiting state")

	if s.waitc == nil {
		s.waitc = time.After(waitingTimeDefault)
	}

	select {
	case <-s.deletec:
		return stateDeleting
	case <-s.waitc:
		return stateReleasing
	}
}
