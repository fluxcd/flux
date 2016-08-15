package automator

import (
	"fmt"
	"time"

	"github.com/weaveworks/fluxy/history"
	"github.com/weaveworks/fluxy/platform/kubernetes"
	"github.com/weaveworks/fluxy/registry"
)

type state string

const (
	stateInvalid    state = "Invalid"
	stateDeleting         = "Deleting"
	stateRefreshing       = "Refreshing"
	stateReleasing        = "Releasing"
	stateWaiting          = "Waiting"
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
	namespace   string
	serviceName string
	st          state
	logf        serviceLogFunc
	waitc       <-chan time.Time // optionally set before moving to Waiting
	candidate   *registry.Image  // set before moving to Releasing
	deletec     chan struct{}    // close to move to Deleting
	onDelete    func()
	cfg         Config
}

func newSvc(namespace, serviceName string, logf serviceLogFunc, onDelete func(), cfg Config) *svc {
	s := &svc{
		namespace:   namespace,
		serviceName: serviceName,
		st:          stateRefreshing,
		logf:        logf,
		waitc:       nil,
		deletec:     make(chan struct{}),
		onDelete:    onDelete,
		cfg:         cfg,
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

		case stateRefreshing:
			s.st = s.refreshing()

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

func (s *svc) refreshing() state {
	s.logf("entering Refreshing state")

	select {
	case <-s.deletec:
		return stateDeleting
	default:
	}

	// Get the service from the platform.
	service, err := s.cfg.Platform.Service(s.namespace, s.serviceName)
	switch err {
	case nil:
		break
	case kubernetes.ErrNoMatchingService:
		s.logf("service doesn't exist; deleting")
		return stateDeleting
	default:
		s.logf("refresh failed when fetching platform service: %v", err)
		return stateWaiting
	}
	s.logf("%s is running %s", service.Name, service.Image)

	if i := registry.ParseImage(service.Image); i.Tag == "" {
		// TODO(pb): service.Image is meant to be a human-readable string, and
		// can be something like "(multiple RCs)". This is a little hack to
		// detect that without doing string comparisons. If you want to fix
		// this, please fix it by making ParseImage return an error, or by
		// making service.Image stricter and signaling the multiple RC condition
		// through some other piece of metadata.
		s.logf("%s is not a valid image (no tag); aborting", service.Image)
		return stateWaiting
	}

	// Get available images based on the current image.
	img := registry.ParseImage(service.Image)
	repo, err := s.cfg.Registry.GetRepository(img.Repository())
	if err != nil {
		s.logf("refresh failed when fetching image repository: %v", err)
		return stateWaiting
	}
	if len(repo.Images) == 0 {
		s.logf("refresh failed when checking image repository: no images found")
		return stateWaiting
	}

	// If we're already running the latest image, we're good.
	if repo.Images[0].String() == service.Image {
		s.logf("refresh successful, already running latest image")
		return stateWaiting
	}

	// Otherwise, we need to be running the latest image.
	s.candidate = &(repo.Images[0])
	s.logf("%s should be running %s", service.Name, s.candidate)
	return stateReleasing
}

func (s *svc) releasing(cfg Config) state {
	s.logf("entering Releasing state")

	// Validate candidate image.
	if s.candidate == nil {
		panic("no candidate available in releasing state; programmer error!")
	}
	defer func() { s.candidate = nil }()
	s.logf("releasing %s", s.candidate)

	if err := s.cfg.Repo.Release(
		s.logf,
		s.cfg.Platform,
		s.namespace,
		s.serviceName,
		*s.candidate,
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
		return stateRefreshing
	}
}
