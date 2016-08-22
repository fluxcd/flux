package flux

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/fluxy/git"
	"github.com/weaveworks/fluxy/history"
	"github.com/weaveworks/fluxy/platform"
	"github.com/weaveworks/fluxy/platform/kubernetes"
	"github.com/weaveworks/fluxy/registry"
)

type Automator interface {
	Automate(namespace, service string) error
	Deautomate(namespace, service string) error
	IsAutomated(namespace, service string) bool
}

type server struct {
	platform  *kubernetes.Cluster
	registry  *registry.Client
	automator Automator
	history   history.DB
	repo      git.Repo
	logger    log.Logger
}

func NewServer(platform *kubernetes.Cluster, registry *registry.Client, automator Automator, history history.DB, repo git.Repo, logger log.Logger) Service {
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

func (s *server) ListServices() (res []ServiceStatus, err error) {
	s.logger.Log("method", "ListServices")
	defer func() { s.logger.Log("method", "ListServices", "res", len(res), "err", err) }()

	serviceIDs, err := s.allServices()
	if err != nil {
		return nil, errors.Wrap(err, "fetching all services on the platform")
	}

	for _, serviceID := range serviceIDs {
		namespace, service := serviceID.Components()

		// TODO(pb): containers should be returned as part of Services
		var c []Container
		containers, err := s.platform.ContainersFor(namespace, service)
		if err != nil {
			s.logger.Log("err", errors.Wrapf(err, "fetching containers for %s", serviceID))
		} else {
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

func (s *server) ListImages(spec ServiceSpec) (res []ImageStatus, err error) {
	s.logger.Log("method", "ListImages", "spec", spec)
	defer func() { s.logger.Log("method", "ListImages", "spec", spec, "res", len(res), "err", err) }()

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

	for _, serviceID := range serviceIDs {
		namespace, service := serviceID.Components()

		var c []Container
		containers, err := s.platform.ContainersFor(namespace, service)
		if err != nil {
			s.logger.Log("err", errors.Wrapf(err, "fetching containers for %s", serviceID))
		} else {
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
		}

		res = append(res, ImageStatus{
			ID:         serviceID,
			Containers: c,
		})
	}

	return res, nil
}

func (s *server) Release(serviceSpec ServiceSpec, imageSpec ImageSpec, kind ReleaseKind) (res []ReleaseAction, err error) {
	s.logger.Log("method", "Release", "serviceSpec", serviceSpec, "imageSpec", imageSpec, "kind", kind)
	defer func() {
		s.logger.Log("method", "Release", "serviceSpec", serviceSpec, "imageSpec", imageSpec, "kind", kind, "res", len(res), "err", err)
	}()

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
	s.automator.Automate(namespace, service)
	return nil
}

func (s *server) Deautomate(id ServiceID) error {
	if s.automator == nil {
		return errors.New("no automator configured")
	}
	namespace, service := id.Components()
	s.automator.Deautomate(namespace, service)
	return nil
}

func (s *server) History(spec ServiceSpec) ([]HistoryEntry, error) {
	var events []history.Event
	if spec == ServiceSpecAll {
		namespaces, err := s.platform.Namespaces()
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

// Specific releaseX functions. The general idea:
// - Walk the platform and collect things to do;
// - If ReleaseKindExecute, execute those things; and then
// - Return the things we did (or didn't) do.

func (s *server) releaseAllToLatest(kind ReleaseKind) (res []ReleaseAction, err error) {
	s.logger.Log("method", "releaseAllToLatest", "kind", kind)
	defer func() { s.logger.Log("method", "releaseAllToLatest", "kind", kind, "res", len(res), "err", err) }()

	res = append(res, s.releaseActionNop("I'm going to release all services to their latest images. Here we go."))

	serviceIDs, err := s.allServices()
	if err != nil {
		return nil, errors.Wrap(err, "fetching all platform services")
	}

	containerMap, err := s.allImagesFor(serviceIDs)
	if err != nil {
		return nil, errors.Wrap(err, "fetching images for services")
	}

	// Each service is running multiple images.
	// Each image may need to be upgraded, and trigger a release.

	regradeMap := map[ServiceID][]containerRegrade{}
	for serviceID, containers := range containerMap {
		for _, container := range containers {
			currentImageID := ParseImageID(container.Image)
			imageRepo, err := s.registry.GetRepository(currentImageID.Repository())
			if err != nil {
				return nil, errors.Wrapf(err, "fetching image repo for %s", currentImageID)
			}
			latestImageID := ParseImageID(imageRepo.Images[0].String())
			if currentImageID != latestImageID {
				regradeMap[serviceID] = append(regradeMap[serviceID], containerRegrade{
					container: container.Name,
					current:   currentImageID,
					target:    latestImageID,
				})
			}
		}
	}
	if len(regradeMap) <= 0 {
		res = append(res, s.releaseActionNop("All services are running the latest images. Nothing to do."))
		return res, nil
	}

	// We have identified at least 1 release that needs to occur. Releasing
	// means cloning the repo, changing the resource file(s), committing and
	// pushing, and then making the release(s) to the platform.

	res = append(res, s.releaseActionClone())
	for service, regrades := range regradeMap {
		res = append(res, s.releaseActionUpdatePodController(service, regrades))
	}
	res = append(res, s.releaseActionCommitAndPush())
	for service := range regradeMap {
		res = append(res, s.releaseActionReleaseService(service))
	}

	if kind == ReleaseKindExecute {
		if err := s.execute(res); err != nil {
			return res, err
		}
	}

	return res, nil
}

func (s *server) releaseAllForImage(target ImageID, kind ReleaseKind) (res []ReleaseAction, err error) {
	s.logger.Log("method", "releaseAllForImage", "kind", kind)
	defer func() { s.logger.Log("method", "releaseAllForImage", "kind", kind, "res", len(res), "err", err) }()

	res = append(res, s.releaseActionNop(fmt.Sprintf("I'm going to release image %s to all services that would use it. Here we go.", target)))

	serviceIDs, err := s.allServices()
	if err != nil {
		return nil, errors.Wrap(err, "fetching all platform services")
	}

	containerMap, err := s.allImagesFor(serviceIDs)
	if err != nil {
		return nil, errors.Wrap(err, "fetching images for services")
	}

	// Each service is running multiple images.
	// Each image may need to be modified, and trigger a release.

	regradeMap := map[ServiceID][]containerRegrade{}
	for serviceID, containers := range containerMap {
		for _, container := range containers {
			candidate := ParseImageID(container.Image)
			if candidate.Repository() == target.Repository() && candidate != target {
				regradeMap[serviceID] = append(regradeMap[serviceID], containerRegrade{
					container: container.Name,
					current:   candidate,
					target:    target,
				})
			}
		}
	}
	if len(regradeMap) <= 0 {
		res = append(res, s.releaseActionNop(fmt.Sprintf("All matching services are already running image %s. Nothing to do.", target)))
		return res, nil
	}

	// We have identified at least 1 release that needs to occur. Releasing
	// means cloning the repo, changing the resource file(s), committing and
	// pushing, and then making the release(s) to the platform.

	res = append(res, s.releaseActionClone())
	for service, imageReleases := range regradeMap {
		res = append(res, s.releaseActionUpdatePodController(service, imageReleases))
	}
	res = append(res, s.releaseActionCommitAndPush())
	for service := range regradeMap {
		res = append(res, s.releaseActionReleaseService(service))
	}

	if kind == ReleaseKindExecute {
		if err := s.execute(res); err != nil {
			return res, err
		}
	}

	return res, nil
}

func (s *server) releaseOneToLatest(id ServiceID, kind ReleaseKind) (res []ReleaseAction, err error) {
	s.logger.Log("method", "releaseOneToLatest", "kind", kind)
	defer func() { s.logger.Log("method", "releaseOneToLatest", "kind", kind, "res", len(res), "err", err) }()

	res = append(res, s.releaseActionNop(fmt.Sprintf("I'm going to release the latest images(s) for service %s. Here we go.", id)))

	namespace, service := id.Components()
	containers, err := s.platform.ContainersFor(namespace, service)
	if err != nil {
		return nil, errors.Wrapf(err, "fetching images for service %s", id)
	}

	// Each service is running multiple images.
	// Each image may need to be modified, and trigger a release.

	var regrades []containerRegrade
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
			regrades = append(regrades, containerRegrade{
				container: container.Name,
				current:   imageID,
				target:    latestID,
			})
		}
	}
	if len(regrades) <= 0 {
		res = append(res, s.releaseActionNop("The service is already running the latest version of all its images. Nothing to do."))
		return res, nil
	}

	// We need to make 1 release. Releasing means cloning the repo, changing the
	// resource file(s), committing and pushing, and then making the release(s)
	// to the platform.

	res = append(res, s.releaseActionClone())
	res = append(res, s.releaseActionUpdatePodController(id, regrades))
	res = append(res, s.releaseActionCommitAndPush())
	res = append(res, s.releaseActionReleaseService(id))

	if kind == ReleaseKindExecute {
		if err := s.execute(res); err != nil {
			return res, err
		}
	}

	return res, nil
}

func (s *server) releaseOne(serviceID ServiceID, target ImageID, kind ReleaseKind) (res []ReleaseAction, err error) {
	s.logger.Log("method", "releaseOneToLatest", "kind", kind)
	defer func() { s.logger.Log("method", "releaseOneToLatest", "kind", kind, "res", len(res), "err", err) }()

	res = append(res, s.releaseActionNop(fmt.Sprintf("I'm going to release image %s to service %s.", target, serviceID)))

	namespace, service := serviceID.Components()
	containers, err := s.platform.ContainersFor(namespace, service)
	if err != nil {
		return nil, errors.Wrapf(err, "fetching images for service %s", serviceID)
	}

	// Each service is running multiple images.
	// Each image may need to be modified, and trigger a release.

	var regrades []containerRegrade
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
	if len(regrades) <= 0 {
		res = append(res, s.releaseActionNop(fmt.Sprintf("All matching containers are already running image %s. Nothing to do.", target)))
		return res, nil
	}

	// We have identified at least 1 regrade that needs to occur. Releasing
	// means cloning the repo, changing the resource file(s), committing and
	// pushing, and then making the release(s) to the platform.

	res = append(res, s.releaseActionClone())
	res = append(res, s.releaseActionUpdatePodController(serviceID, regrades))
	res = append(res, s.releaseActionCommitAndPush())
	res = append(res, s.releaseActionReleaseService(serviceID))

	if kind == ReleaseKindExecute {
		if err := s.execute(res); err != nil {
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

type containerRegrade struct {
	container string
	current   ImageID
	target    ImageID
}

func (s *server) allServices() ([]ServiceID, error) {
	namespaces, err := s.platform.Namespaces()
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

// ReleaseAction Do funcs

func (s *server) releaseActionNop(desc string) ReleaseAction {
	return ReleaseAction{Description: desc}
}

func (s *server) releaseActionClone() ReleaseAction {
	return ReleaseAction{
		Description: "Clone the config repo.",
		Do: func(rc *ReleaseContext) error {
			path, keyFile, err := s.repo.Clone()
			if err != nil {
				return errors.Wrap(err, "clone the config repo")
			}
			rc.RepoPath = path
			rc.RepoKey = keyFile
			return nil
		},
	}
}

func (s *server) releaseActionUpdatePodController(service ServiceID, regrades []containerRegrade) ReleaseAction {
	var actions []string
	for _, regrade := range regrades {
		actions = append(actions, fmt.Sprintf("%s (%s -> %s)", regrade.container, regrade.current, regrade.target))
	}
	actionList := strings.Join(actions, ", ")

	return ReleaseAction{
		Description: fmt.Sprintf("Update %d images(s) in the resource definition file for %s: %s.", len(regrades), service, actionList),
		Do: func(rc *ReleaseContext) error {
			if fi, err := os.Stat(rc.RepoPath); err != nil || !fi.IsDir() {
				return fmt.Errorf("the repo path (%s) is not valid", rc.RepoPath)
			}

			namespace, serviceName := service.Components()
			files, err := kubernetes.FilesFor(rc.RepoPath, namespace, serviceName)
			s.logger.Log("DEBUG", "###", "after", "FilesFor", "files", strings.Join(files, ", "), "err", err)
			if err != nil {
				return errors.Wrapf(err, "finding resource definition file for %s", service)
			}
			if len(files) <= 0 {
				return fmt.Errorf("no resource definition file found for %s", service)
			}
			if len(files) > 1 {
				return fmt.Errorf("multiple resource definition files found for %s: %s", service, strings.Join(files, ", "))
			}

			def, err := ioutil.ReadFile(files[0])
			if err != nil {
				return err
			}
			fi, err := os.Stat(files[0])
			if err != nil {
				return err
			}

			for _, regrade := range regrades {
				// Note 1: UpdatePodController parses the target (new) image
				// name, extracts the repository, and only mutates the line(s)
				// in the definition that match it. So for the time being we
				// ignore the current image. UpdatePodController could be
				// updated, if necessary.
				//
				// Note 2: we keep overwriting the same def, to handle multiple
				// images in a single file.
				def, err = kubernetes.UpdatePodController(def, string(regrade.target), ioutil.Discard)
				if err != nil {
					return errors.Wrapf(err, "updating pod controller for %s", regrade.target)
				}
			}

			// Write the file back, so commit/push works.
			if err := ioutil.WriteFile(files[0], def, fi.Mode()); err != nil {
				return err
			}

			// Put the def in the map, so release works.
			rc.PodControllers[service] = def
			return nil
		},
	}
}

func (s *server) releaseActionCommitAndPush() ReleaseAction {
	return ReleaseAction{
		Description: "Commit and push the config repo.",
		Do: func(rc *ReleaseContext) error {
			if fi, err := os.Stat(rc.RepoPath); err != nil || !fi.IsDir() {
				return fmt.Errorf("the repo path (%s) is not valid", rc.RepoPath)
			}
			if _, err := os.Stat(rc.RepoKey); err != nil {
				return fmt.Errorf("the repo key (%s) is not valid: %v", rc.RepoKey, err)
			}
			return s.repo.CommitAndPush(rc.RepoPath, rc.RepoKey, "Fluxy release")
		},
	}
}

func (s *server) releaseActionReleaseService(service ServiceID) ReleaseAction {
	return ReleaseAction{
		Description: fmt.Sprintf("Release the service %s.", service),
		Do: func(rc *ReleaseContext) error {
			def, ok := rc.PodControllers[service]
			if !ok {
				return errors.New("didn't find pod controller definition for " + string(service))
			}
			namespace, serviceName := service.Components()
			return s.platform.Release(namespace, serviceName, def)
		},
	}
}
