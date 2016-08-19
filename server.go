package flux

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/fluxy/automator"
	"github.com/weaveworks/fluxy/git"
	"github.com/weaveworks/fluxy/history"
	"github.com/weaveworks/fluxy/platform"
	"github.com/weaveworks/fluxy/platform/kubernetes"
	"github.com/weaveworks/fluxy/registry"
)

type server struct {
	platform  *kubernetes.Cluster
	registry  *registry.Client
	automator *automator.Automator
	history   history.DB
	repo      git.Repo
	logger    log.Logger
}

func NewServer(platform *kubernetes.Cluster, registry *registry.Client, automator *automator.Automator, history history.DB, repo git.Repo, logger log.Logger) Service {
	return &server{
		platform:  platform,
		registry:  registry,
		automator: automator,
		history:   history,
		repo:      repo,
		logger:    logger,
	}
}

// The server methods are deliberately awkward, cobbled together from existing
// platform and registry APIs. I want to avoid changing those components until I
// get something working. There's also a lot of code duplication here for the
// same reason: let's not add abstraction until it's merged, or nearly so, and
// it's clear where the abstraction should exist.

func (s *server) ListServices() ([]ServiceStatus, error) {
	serviceIDs, err := s.allServices()
	if err != nil {
		return nil, errors.Wrap(err, "fetching all services on the platform")
	}

	var res []ServiceStatus
	for _, serviceID := range serviceIDs {
		namespace, service := serviceID.Components()

		// TODO(pb): containers should be returned as part of Services
		containers, err := s.platform.ContainersFor(namespace, service)
		if err != nil {
			return nil, errors.Wrapf(err, "fetching containers for %s", serviceID)
		}

		var c []Container
		for _, container := range containers {
			imageID := ParseImageID(container.Image)
			imageRepo, err := s.registry.GetRepository(imageID.Repository())
			if err != nil {
				return nil, errors.Wrapf(err, "fetching image repo for %s", imageID)
			}

			var (
				current   ImageDescription
				available []ImageDescription
			)
			for _, image := range imageRepo.Images {
				description := ImageDescription{
					ID:        ParseImageID(image.String()),
					CreatedAt: image.CreatedAt,
				}
				available = append(available, description)
				if image.String() == container.Image {
					current = description
				}
			}
			c = append(c, Container{
				Name:      container.Name,
				Current:   current,
				Available: available,
			})
		}

		// FIXME: since we get service IDs above, we have to get the
		// service; but this is repeating work, since to get the
		// service IDs we got a list of all the services ...
		platformSvc, err := s.platform.Service(namespace, service)
		if err != nil {
			return nil, errors.Wrapf(err, "getting platform service %s", serviceID)
		}

		res = append(res, ServiceStatus{
			ID:         serviceID,
			Containers: c,
			Status:     platformSvc.Status,
			Automated:  s.automator.IsAutomated(namespace, service),
		})
	}

	return res, nil
}

func (s *server) ListImages(spec ServiceSpec) ([]ImageStatus, error) {
	serviceIDs, err := func() ([]ServiceID, error) {
		if spec == ServiceSpecAll {
			return s.allServices()
		}
		id, err := ParseServiceID(string(spec))
		return []ServiceID{id}, err
	}()
	if err != nil {
		return nil, errors.Wrapf(err, "fetching service ID(s)")
	}

	var res []ImageStatus
	for _, serviceID := range serviceIDs {
		namespace, service := serviceID.Components()
		containers, err := s.platform.ContainersFor(namespace, service)
		if err != nil {
			return nil, errors.Wrapf(err, "fetching containers for %s", serviceID)
		}

		var c []Container
		for _, container := range containers {
			imageID := ParseImageID(container.Image)
			imageRepo, err := s.registry.GetRepository(imageID.Repository())
			if err != nil {
				return nil, errors.Wrapf(err, "fetching image repo for %s", imageID)
			}

			var (
				current   ImageDescription
				available []ImageDescription
			)
			for _, image := range imageRepo.Images {
				description := ImageDescription{
					ID:        ParseImageID(image.String()),
					CreatedAt: image.CreatedAt,
				}
				available = append(available, description)
				if image.String() == container.Image {
					current = description
				}
			}
			c = append(c, Container{
				Name:      container.Name,
				Current:   current,
				Available: available,
			})
		}

		res = append(res, ImageStatus{
			ID:         serviceID,
			Containers: c,
		})
	}

	return res, nil
}

func (s *server) Release(serviceSpec ServiceSpec, imageSpec ImageSpec, kind ReleaseKind) ([]ReleaseAction, error) {
	switch {
	case serviceSpec == ServiceSpecAll && imageSpec == ImageSpecLatest:
		return s.releaseAllToLatest(kind)

	case serviceSpec == ServiceSpecAll:
		imageID := ParseImageID(string(imageSpec))
		return s.releaseAllForImage(imageID, kind)

	case imageSpec == ImageSpecLatest:
		serviceID, err := ParseServiceID(string(serviceSpec))
		if err != nil {
			return nil, errors.Wrapf(err, "parsing service ID from spec %s", serviceSpec)
		}
		return s.releaseOneToLatest(serviceID, kind)

	default:
		serviceID, err := ParseServiceID(string(serviceSpec))
		if err != nil {
			return nil, errors.Wrapf(err, "parsing service ID from spec %s", serviceSpec)
		}
		imageID := ParseImageID(string(imageSpec))
		return s.releaseOne(serviceID, imageID, kind)
	}
}

func (s *server) Automate(id ServiceID) error {
	if s.automator == nil {
		return errors.New("no automator configured")
	}
	namespace, service := id.Components()
	s.automator.Enable(namespace, service)
	return nil
}

func (s *server) Deautomate(id ServiceID) error {
	if s.automator == nil {
		return errors.New("no automator configured")
	}
	namespace, service := id.Components()
	s.automator.Disable(namespace, service)
	return nil
}

func (s *server) History(spec ServiceSpec) ([]HistoryEntry, error) {
	var events []history.Event
	if spec == ServiceSpecAll {
		namespaces, err := s.allNamespaces()
		if err != nil {
			return nil, errors.Wrap(err, "fetching platform namespaces")
		}

		for _, namespace := range namespaces {
			ev, err := s.history.AllEvents(namespace)
			if err != nil {
				return nil, errors.Wrapf(err, "fetching all history events for namespace %s", namespace)
			}

			events = append(events, ev...)
		}
	} else {
		id, err := ParseServiceID(string(spec))
		if err != nil {
			return nil, errors.Wrapf(err, "parsing service ID from spec %s", spec)
		}

		namespace, service := id.Components()
		ev, err := s.history.EventsForService(namespace, service)
		if err != nil {
			return nil, errors.Wrapf(err, "fetching history events for %s", id)
		}

		events = append(events, ev...)
	}

	res := make([]HistoryEntry, len(events))
	for i, event := range events {
		res[i] = HistoryEntry{
			Stamp: event.Stamp,
			Type:  "v0",
			Data:  fmt.Sprintf("%s: %s", event.Service, event.Msg),
		}
	}

	return res, nil
}

// The general idea for the releaseX functions:
// - Walk the platform and collect things to do;
// - If ReleaseKindExecute, execute those things; and then
// - Return the things we did (or didn't) do.
//
// Every ReleaseAction will need to be extended with a function
// that does the thing that's described.

func (s *server) releaseAllToLatest(kind ReleaseKind) ([]ReleaseAction, error) {
	res := []ReleaseAction{ReleaseAction{
		Description: "I'm going to release all services to their latest images. Here we go.",
	}}

	// Walk all services on the platform.
	serviceIDs, err := s.allServices()
	if err != nil {
		return nil, errors.Wrap(err, "fetching all platform services")
	}

	// Fetch all of the images that each service is running.
	containerMap, err := s.allImagesFor(serviceIDs)
	if err != nil {
		return nil, errors.Wrap(err, "fetching images for services")
	}

	// Each service is running multiple images.
	// Each image may need to be upgraded, and trigger a release.
	type imageRelease struct {
		current ImageID
		target  ImageID
	}
	releaseMap := map[ServiceID][]imageRelease{}
	for serviceID, containers := range containerMap {
		for _, container := range containers {
			currentImageID := ParseImageID(container.Image)
			imageRepo, err := s.registry.GetRepository(currentImageID.Repository())
			if err != nil {
				return nil, errors.Wrapf(err, "fetching image repo for %s", currentImageID)
			}
			latestImageID := ParseImageID(imageRepo.Images[0].String())
			if currentImageID != latestImageID {
				releaseMap[serviceID] = append(releaseMap[serviceID], imageRelease{current: currentImageID, target: latestImageID})
			}
		}
	}

	// If no services need updates, we're done.
	if len(releaseMap) <= 0 {
		res = append(res, ReleaseAction{
			Description: "All services are running the latest images. Nothing to do.",
		})
		return res, nil
	}

	// We have identified at least 1 release that needs to occur. Releasing
	// means cloning the repo, changing the resource file(s), committing and
	// pushing, and then making the release(s) to the platform.
	res = append(res, ReleaseAction{
		Description: "Clone the config repo.",
		Do: func(rc *ReleaseContext) error {
			path, err := s.repo.Clone()
			if err != nil {
				return errors.Wrap(err, "clone the config repo")
			}

			rc.RepoPath = path
			return nil
		},
	})

	// We will first make all of the file changes, commit, and push.
	for service, imageReleases := range releaseMap {
		var changes []string
		for _, imageRelease := range imageReleases {
			changes = append(changes, fmt.Sprintf("%s -> %s", imageRelease.current, imageRelease.target))
		}
		res = append(res, ReleaseAction{
			Description: fmt.Sprintf("Make %d change(s) to the resource file for %s: %s", len(changes), service, strings.Join(changes, ", ")),
			Do: func(rc *ReleaseContext) error {
				if fi, err := os.Stat(rc.RepoPath); err != nil || !fi.IsDir() {
					return fmt.Errorf("the repo path (%s) is not valid", rc.RepoPath)
				}

				file, err := fileFor(rc.RepoPath, service)
				if err != nil {
					return errors.Wrapf(err, "finding resource definition file for %s", service)
				}

				def, err := ioutil.ReadFile(file)
				if err != nil {
					return err
				}

				for _, release := range imageReleases {
					// TODO(pb) this is insufficient; UpdatePodController needs
					// to take the old AND new image name.. !
					//
					// Note: keep overwriting the same def, to handle multiple
					// images in a single file.
					def, err = kubernetes.UpdatePodController(def, string(release.target), ioutil.Discard)
					if err != nil {
						return errors.Wrapf(err, "updating pod controller for %s", release.target)
					}
				}

				rc.PodControllers[service] = def
				return nil
			},
		})
	}
	res = append(res, ReleaseAction{
		Description: "Commit and push the config repo.",
		Do: func(rc *ReleaseContext) error {
			if fi, err := os.Stat(rc.RepoPath); err != nil || !fi.IsDir() {
				return fmt.Errorf("the repo path (%s) is not valid", rc.RepoPath)
			}

			return s.repo.CommitAndPush(rc.RepoPath, "Fluxy release")
		},
	})

	// Then, we will make all of the releases serially.
	for service := range releaseMap {
		res = append(res, ReleaseAction{
			Description: fmt.Sprintf("Release the service %s.", service),
			Do: func(rc *ReleaseContext) error {
				def, ok := rc.PodControllers[service]
				if !ok {
					return errors.New("didn't find pod controller definition for " + string(service))
				}
				namespace, serviceName := service.Components()
				return s.platform.Release(namespace, serviceName, def)
			},
		})
	}

	if kind == ReleaseKindExecute {
		if err := s.execute(res); err != nil {
			return res, err
		}
	}

	return res, nil
}

func (s *server) releaseAllForImage(target ImageID, kind ReleaseKind) ([]ReleaseAction, error) {
	res := []ReleaseAction{ReleaseAction{
		Description: fmt.Sprintf("I'm going to release image %s to all services that would use it. Here we go.", target),
	}}

	// Walk all services on the platform.
	serviceIDs, err := s.allServices()
	if err != nil {
		return nil, errors.Wrap(err, "fetching all platform services")
	}

	// Fetch all of the images that each service is running.
	containerMap, err := s.allImagesFor(serviceIDs)
	if err != nil {
		return nil, errors.Wrap(err, "fetching images for services")
	}

	// Each service is running multiple images.
	// Each image may need to be modified, and trigger a release.
	type imageRelease struct {
		current ImageID
		target  ImageID
	}
	releaseMap := map[ServiceID][]imageRelease{}
	for serviceID, containers := range containerMap {
		for _, container := range containers {
			candidate := ParseImageID(container.Image)
			if candidate.Repository() == target.Repository() && candidate != target {
				releaseMap[serviceID] = append(releaseMap[serviceID], imageRelease{current: candidate, target: target})
			}
		}
	}

	// If no services need updates, we're done.
	if len(releaseMap) <= 0 {
		res = append(res, ReleaseAction{
			Description: fmt.Sprintf("All matching services are already running image %s. Nothing to do.", target),
		})
		return res, nil
	}

	// (From here, this is a straight copy/paste from the above.)

	// We have identified at least 1 release that needs to occur. Releasing
	// means cloning the repo, changing the resource file(s), committing and
	// pushing, and then making the release(s) to the platform.
	res = append(res, ReleaseAction{
		Description: "Clone the config repo.",
	})

	// We will first make all of the file changes, commit, and push.
	for service, imageReleases := range releaseMap {
		var changes []string
		for _, imageRelease := range imageReleases {
			changes = append(changes, fmt.Sprintf("%s -> %s", imageRelease.current, imageRelease.target))
		}
		res = append(res, ReleaseAction{
			Description: fmt.Sprintf("Make %d change(s) to the resource file for %s: %s", len(changes), service, strings.Join(changes, ", ")),
		})
	}
	res = append(res, ReleaseAction{
		Description: "Commit and push the config repo.",
	})

	// Then, we will make all of the releases serially.
	for service := range releaseMap {
		res = append(res, ReleaseAction{
			Description: fmt.Sprintf("Release the service %s.", service),
		})
	}

	if kind == ReleaseKindExecute {
		if err := s.execute(res); err != nil {
			return res, err
		}
	}

	return res, nil
}

func (s *server) releaseOneToLatest(id ServiceID, kind ReleaseKind) ([]ReleaseAction, error) {
	res := []ReleaseAction{ReleaseAction{
		Description: fmt.Sprintf("I'm going to release the latest images(s) for service %s. Here we go.", id),
	}}

	// Fetch the images for the service.
	namespace, service := id.Components()
	containers, err := s.platform.ContainersFor(namespace, service)
	if err != nil {
		return nil, errors.Wrapf(err, "fetching images for service %s", id)
	}

	// Each service is running multiple images.
	// Each image may need to be modified, and trigger a release.
	type imageRelease struct {
		current ImageID
		target  ImageID
	}
	var releases []imageRelease
	for _, container := range containers {
		imageID := ParseImageID(container.Image)
		imageRepo, err := s.registry.GetRepository(imageID.Repository())
		if err != nil {
			return nil, errors.Wrapf(err, "fetching repository for %s", imageID)
		}
		if len(imageRepo.Images) <= 0 {
			continue // strange
		}

		latestID := ParseImageID(imageRepo.Images[0].String())
		if imageID != latestID {
			releases = append(releases, imageRelease{current: imageID, target: latestID})
		}
	}

	// If the service doesn't need an update, we're done.
	if len(releases) <= 0 {
		res = append(res, ReleaseAction{
			Description: "The service is already running the latest version of all its images. Nothing to do.",
		})
		return res, nil
	}

	// (From here, this is *almost* a straight copy/paste from above.)

	// We need to make 1 release. Releasing means cloning the repo, changing the
	// resource file(s), committing and pushing, and then making the release(s)
	// to the platform.
	res = append(res, ReleaseAction{
		Description: "Clone the config repo.",
	})

	// We will first make the changes to the file, commit, and push.
	var changes []string
	for _, release := range releases {
		changes = append(changes, fmt.Sprintf("%s -> %s", release.current, release.target))
	}
	res = append(res, ReleaseAction{
		Description: fmt.Sprintf("Make %d change(s) to the resource file for %s: %s", len(changes), service, strings.Join(changes, ", ")),
	})
	res = append(res, ReleaseAction{
		Description: "Commit and push the config repo.",
	})

	// Then, we will make the release.
	res = append(res, ReleaseAction{
		Description: fmt.Sprintf("Release the service %s.", id),
	})

	if kind == ReleaseKindExecute {
		if err := s.execute(res); err != nil {
			return res, err
		}
	}

	return res, nil
}

func (s *server) releaseOne(serviceID ServiceID, target ImageID, kind ReleaseKind) ([]ReleaseAction, error) {
	res := []ReleaseAction{ReleaseAction{
		Description: fmt.Sprintf("I'm going to release image %s to service %s.", target, serviceID),
	}}

	namespace, service := serviceID.Components()
	containers, err := s.platform.ContainersFor(namespace, service)
	if err != nil {
		return nil, errors.Wrapf(err, "fetching images for service %s", serviceID)
	}

	// Each service is running multiple images.
	// Each image may need to be modified, and trigger a release.
	type containerRegrade struct {
		container string
		current   ImageID
		target    ImageID
	}
	regrades := []containerRegrade{}

	for _, container := range containers {
		candidate := ParseImageID(container.Image)
		if candidate.Repository() == target.Repository() && candidate != target {
			regrades = append(regrades, containerRegrade{
				container: container.Name,
				current:   candidate,
				target:    target,
			})
		}
	}

	// If no services need updates, we're done.
	if len(regrades) <= 0 {
		res = append(res, ReleaseAction{
			Description: fmt.Sprintf("All matching containers are already running image %s. Nothing to do.", target),
		})
		return res, nil
	}

	// We have identified at least 1 regrade that needs to occur. Releasing
	// means cloning the repo, changing the resource file(s), committing and
	// pushing, and then making the release(s) to the platform.
	res = append(res, ReleaseAction{
		Description: "Clone the config repo.",
	})

	var changes []string
	for _, regrade := range regrades {
		changes = append(changes, fmt.Sprintf("%s: %s -> %s", regrade.container, regrade.current, regrade.target))
	}
	res = append(res, ReleaseAction{
		Description: fmt.Sprintf("Make %d change(s) to the resource file for %s: %s", len(changes), service, strings.Join(changes, ", ")),
	})

	res = append(res, ReleaseAction{
		Description: "Commit and push the config repo.",
	})

	res = append(res, ReleaseAction{
		Description: fmt.Sprintf("Release the service %s.", service),
	})

	if kind == ReleaseKindExecute {
		if err := execute(res); err != nil {
			return res, err
		}
	}

	return res, nil
}

func (s *server) execute(actions []ReleaseAction) error {
	rc := newReleaseContext()
	defer rc.Clean()

	for _, action := range actions {
		s.logger.Log("description", action.Description)
		if action.Do == nil {
			continue
		}

		if err := action.Do(rc); err != nil {
			s.logger.Log("err", err)
			return err
		}
	}

	return nil
}

// Release helpers.

func (s *server) allNamespaces() ([]string, error) {
	return []string{"default"}, nil // TODO(pb): s.platform.Namespaces()
}

func (s *server) allServices() ([]ServiceID, error) {
	namespaces, err := s.allNamespaces()
	if err != nil {
		return nil, errors.Wrap(err, "fetching platform namespaces")
	}

	var res []ServiceID
	for _, namespace := range namespaces {
		services, err := s.platform.Services(namespace)
		if err != nil {
			return nil, errors.Wrapf(err, "fetching platform services for namespace %q", namespace)
		}

		for _, service := range services {
			idStr := fmt.Sprintf("%s/%s", namespace, service.Name)
			id, err := ParseServiceID(idStr)
			if err != nil {
				return nil, errors.Wrapf(err, "parsing service ID %q", idStr)
			}

			res = append(res, id)
		}
	}

	return res, nil
}

func (s *server) allImagesFor(serviceIDs []ServiceID) (map[ServiceID][]platform.Container, error) {
	containerMap := map[ServiceID][]platform.Container{}
	for _, serviceID := range serviceIDs {
		namespace, service := serviceID.Components()
		containers, err := s.platform.ContainersFor(namespace, service)
		if err != nil {
			return nil, errors.Wrapf(err, "fetching containers for %s", serviceID)
		}
		if len(containers) <= 0 {
			continue
		}
		containerMap[serviceID] = containers
	}
	return containerMap, nil
}
