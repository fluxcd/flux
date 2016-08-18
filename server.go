package flux

import (
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"

	"github.com/weaveworks/fluxy/automator"
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
}

func NewServer(platform *kubernetes.Cluster, registry *registry.Client, automator *automator.Automator, history history.DB) Service {
	return &server{
		platform:  platform,
		registry:  registry,
		automator: automator,
		history:   history,
	}
}

// The server methods are deliberately awkward, cobbled together from existing
// platform and registry APIs. I want to avoid changing those components until I
// get something working. There's also a lot of code duplication here for the
// same reason: let's not add abstraction until it's merged, or nearly so, and
// it's clear where the abstraction should exist.

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

func (s *server) ListServices() ([]ServiceDescription, error) {
	serviceIDs, err := s.allServices()
	if err != nil {
		return nil, errors.Wrap(err, "fetching all services on the platform")
	}

	var res []ServiceDescription
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

		res = append(res, ServiceDescription{
			ID:         serviceID,
			Containers: c,
		})
	}

	return res, nil
}

func (s *server) ListImages(spec ServiceSpec) ([]ImageDescription, error) {
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

	var res []ImageDescription
	for _, serviceID := range serviceIDs {
		namespace, service := serviceID.Components()
		containers, err := s.platform.ContainersFor(namespace, service)
		if err != nil {
			return nil, errors.Wrapf(err, "fetching containers for %s", serviceID)
		}
		for _, container := range containers {
			imageID := ParseImageID(container.Image)
			imageRepo, err := s.registry.GetRepository(imageID.Repository())
			if err != nil {
				return nil, errors.Wrapf(err, "fetching image repo for %s", imageID)
			}
			for _, image := range imageRepo.Images {
				res = append(res, ImageDescription{
					ID:        ParseImageID(image.String()),
					CreatedAt: image.CreatedAt,
				})
			}
		}
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
// - If ReleaseKindExecute, do them sequentially; and then
// - Return the things we did (or didn't) do.

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
			Description: "All services running latest images. Nothing to do.",
		})
		return res, nil
	}

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
		if err := execute(res); err != nil {
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
		if err := execute(res); err != nil {
			return res, err
		}
	}

	return res, nil
}

func (s *server) releaseOneToLatest(id ServiceID, kind ReleaseKind) ([]ReleaseAction, error) {
	return nil, errors.New("releaseOneToLatest not implemented in server")
}

func (s *server) releaseOne(serviceID ServiceID, imageID ImageID, kind ReleaseKind) ([]ReleaseAction, error) {
	return nil, errors.New("releaseOne not implemented in server")
}

func execute(actions []ReleaseAction) error {
	for _, action := range actions {
		fmt.Fprintf(os.Stdout, "Executing: %s\n", action.Description) // TODO(pb)
	}
	return nil
}
