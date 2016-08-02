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

type serviceLogger func(msg string)

func makeServiceLogger(his history.DB, namespace, serviceName string) serviceLogger {
	return func(msg string) { his.LogEvent(namespace, serviceName, "Automation: "+msg) }
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
	k8s         *kubernetes.Cluster
	reg         *registry.Client
	log         serviceLogger
	waitc       <-chan time.Time // optionally set before moving to Waiting
	candidate   *registry.Image  // set before moving to Releasing
	deletec     chan struct{}    // close to move to Deleting
	onDelete    func()
}

func newSvc(namespace, serviceName string, k8s *kubernetes.Cluster, reg *registry.Client, log serviceLogger, onDelete func()) *svc {
	s := &svc{
		namespace:   namespace,
		serviceName: serviceName,
		st:          stateRefreshing,
		k8s:         k8s,
		reg:         reg,
		log:         log,
		waitc:       nil,
		deletec:     make(chan struct{}),
		onDelete:    onDelete,
	}
	go s.loop()
	return s
}

// See doc.go for an illustration of the service state machine.
func (s *svc) loop() {
	s.log("service activated")
	defer s.log("service deactivated")

	for {
		switch s.st {
		case stateDeleting:
			s.onDelete()
			return

		case stateRefreshing:
			s.st = s.refreshing()

		case stateReleasing:
			s.st = s.releasing()

		case stateWaiting:
			s.st = s.waiting()
		}
	}
}

func (s *svc) signalDelete() {
	s.log("entering Deleting state")
	close(s.deletec)
}

func (s *svc) refreshing() state {
	s.log("entering Refreshing state")

	select {
	case <-s.deletec:
		return stateDeleting
	default:
	}

	// Get the service from the platform.
	service, err := s.k8s.Service(s.namespace, s.serviceName)
	switch err {
	case nil:
		break
	case kubernetes.ErrNoMatchingService:
		s.log("service doesn't exist; deleting")
		return stateDeleting
	default:
		s.log(fmt.Sprintf("refresh failed when fetching platform service: %v", err))
		return stateWaiting
	}
	s.log(fmt.Sprintf("%s is running %s", service.Name, service.Image))

	// Get available images based on the current image.
	img := registry.ParseImage(service.Image)
	repo, err := s.reg.GetRepository(img.Repository())
	if err != nil {
		s.log(fmt.Sprintf("refresh failed when fetching image repository: %v", err))
		return stateWaiting
	}
	if len(repo.Images) == 0 {
		s.log("refresh failed when checking image repository: no images found")
		return stateWaiting
	}

	// If we're already running the latest image, we're good.
	if repo.Images[0].String() == service.Image {
		s.log("refresh successful, already running latest image")
		return stateWaiting
	}

	// Otherwise, we need to be running the latest image.
	s.candidate = &(repo.Images[0])
	s.log(fmt.Sprintf("%s should be running %s", service.Name, s.candidate))
	return stateReleasing
}

func (s *svc) releasing() state {
	s.log("entering Releasing state")

	if s.candidate == nil {
		panic("no candidate available in releasing state; programmer error!")
	}
	defer func() { s.candidate = nil }()

	s.log(fmt.Sprintf("releasing %s", s.candidate))

	// Check out latest version of config repo
	s.log("TODO: checking out latest version of config repo") // TODO(pb)

	// Find the relevant RC
	s.log("TODO: identifying relevant resource definitions") // TODO(pb)

	// Mutate it to the right version
	s.log("TODO: mutating relevant resource definitions to the new image") // TODO(pb)

	var err error // = s.k8s.Release(s.namespace, s.serviceName, rc, s.updatePeriod) // TODO(pb)
	if err != nil {
		s.log(fmt.Sprintf("release failed: %v", err))
		return stateWaiting
	}

	// Write the mutated file to disk
	s.log("TODO: writing mutated resource definitions to disk") // TODO(pb)

	// Commit and push the config repo
	s.log("TODO: committing and pushing config repo") // TODO(pb)
	time.Sleep(5 * time.Second)

	s.log("TODO: service release succeeded")
	return stateWaiting
}

func (s *svc) waiting() state {
	s.log("entering Waiting state")

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
