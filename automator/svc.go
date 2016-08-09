package automator

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"path/filepath"

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

	// Check out latest version of config repo.
	s.logf("fetching config repo")
	configPath, err := gitClone(cfg.ConfigRepoKey, cfg.ConfigRepoURL)
	if err != nil {
		s.logf("clone of config repo failed: %v", err)
		return stateWaiting
	}
	defer os.RemoveAll(configPath)

	// Find the relevant resource definition file.
	file, err := findFileFor(configPath, cfg.ConfigRepoPath, s.candidate.Repository())
	if err != nil {
		s.logf("couldn't find a resource definition file: %v", err)
		return stateWaiting
	}

	// Special case: will this actually result in an update?
	if fileContains(file, s.candidate.String()) {
		s.logf("%s already set to %s; no release necessary", filepath.Base(file), s.candidate.String())
		return stateWaiting
	}

	// Mutate the file so it points to the right image.
	// TODO(pb): should validate file contents are what we expect.
	if err := configUpdate(file, s.candidate.String()); err != nil {
		s.logf("config update failed: %v", err)
		return stateWaiting
	}

	// Make the release.
	buf, err := ioutil.ReadFile(file)
	if err != nil {
		s.logf("couldn't read the resource definition file: %v", err)
		return stateWaiting
	}
	s.logf("starting release...")
	err = s.cfg.Platform.Release(s.namespace, s.serviceName, buf, cfg.UpdatePeriod)
	if err != nil {
		s.logf("release failed: %v", err)
		return stateWaiting
	}
	s.logf("release complete")

	// Commit and push the mutated file.
	if err := gitCommitAndPush(cfg.ConfigRepoKey, configPath, "Automated deployment of "+s.candidate.String()); err != nil {
		s.logf("commit and push failed: %v", err)
		return stateWaiting
	}
	s.logf("committed and pushed the resource definition file %s", file)

	s.logf("service release succeeded")
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
