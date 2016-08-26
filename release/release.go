package release

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/fluxy"
	"github.com/weaveworks/fluxy/git"
	"github.com/weaveworks/fluxy/platform/kubernetes"
	"github.com/weaveworks/fluxy/registry"
)

type releaser struct {
	helper flux.Helper
	repo   git.Repo
}

func New(platform *kubernetes.Cluster, registry *registry.Client, logger log.Logger, repo git.Repo) *releaser {
	return &releaser{
		helper: flux.Helper{
			Platform: platform,
			Registry: registry,
			Logger:   logger,
		},
		repo: repo,
	}
}

func (s *releaser) Release(serviceSpec flux.ServiceSpec, imageSpec flux.ImageSpec, kind flux.ReleaseKind) (res []flux.ReleaseAction, err error) {
	s.helper.Log("method", "Release", "serviceSpec", serviceSpec, "imageSpec", imageSpec, "kind", kind)
	defer func() {
		s.helper.Log("method", "Release", "serviceSpec", serviceSpec, "imageSpec", imageSpec, "kind", kind, "res", len(res), "err", err)
	}()

	switch {
	case serviceSpec == flux.ServiceSpecAll && imageSpec == flux.ImageSpecLatest:
		return s.releaseAllToLatest(kind)

	case serviceSpec == flux.ServiceSpecAll && imageSpec == flux.ImageSpecNone:
		return s.releaseAllWithoutUpdate(kind)

	case serviceSpec == flux.ServiceSpecAll:
		imageID := flux.ParseImageID(string(imageSpec))
		return s.releaseAllForImage(imageID, kind)

	case imageSpec == flux.ImageSpecLatest:
		serviceID, err := flux.ParseServiceID(string(serviceSpec))
		if err != nil {
			return nil, errors.Wrapf(err, "parsing service ID from spec %s", serviceSpec)
		}
		return s.releaseOneToLatest(serviceID, kind)

	case imageSpec == flux.ImageSpecNone:
		serviceID, err := flux.ParseServiceID(string(serviceSpec))
		if err != nil {
			return nil, errors.Wrapf(err, "parsing service ID from spec %s", serviceSpec)
		}
		return s.releaseOneWithoutUpdate(serviceID, kind)

	default:
		serviceID, err := flux.ParseServiceID(string(serviceSpec))
		if err != nil {
			return nil, errors.Wrapf(err, "parsing service ID from spec %s", serviceSpec)
		}
		imageID := flux.ParseImageID(string(imageSpec))
		return s.releaseOne(serviceID, imageID, kind)
	}
}

// Specific releaseX functions. The general idea:
// - Walk the platform and collect things to do;
// - If ReleaseKindExecute, execute those things; and then
// - Return the things we did (or didn't) do.

func (s *releaser) releaseAllToLatest(kind flux.ReleaseKind) (res []flux.ReleaseAction, err error) {
	s.helper.Log("method", "releaseAllToLatest", "kind", kind)
	defer func() { s.helper.Log("method", "releaseAllToLatest", "kind", kind, "res", len(res), "err", err) }()

	res = append(res, s.releaseActionNop("I'm going to release all services to their latest images. Here we go."))

	serviceIDs, err := s.helper.AllServices()
	if err != nil {
		return nil, errors.Wrap(err, "fetching all platform services")
	}

	containerMap, err := s.helper.AllImagesFor(serviceIDs)
	if err != nil {
		return nil, errors.Wrap(err, "fetching images for services")
	}

	// Each service is running multiple images.
	// Each image may need to be upgraded, and trigger a release.

	regradeMap := map[flux.ServiceID][]containerRegrade{}
	for serviceID, containers := range containerMap {
		for _, container := range containers {
			currentImageID := flux.ParseImageID(container.Image)
			imageRepo, err := s.helper.Registry.GetRepository(currentImageID.Repository())
			if err != nil {
				return nil, errors.Wrapf(err, "fetching image repo for %s", currentImageID)
			}
			latestImageID := flux.ParseImageID(imageRepo.Images[0].String())
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
	res = append(res, s.releaseActionCommitAndPush("Release latest images to all services"))
	for service := range regradeMap {
		res = append(res, s.releaseActionReleaseService(service))
	}

	if kind == flux.ReleaseKindExecute {
		if err := s.execute(res); err != nil {
			return res, err
		}
	}

	return res, nil
}

func (s *releaser) releaseAllForImage(target flux.ImageID, kind flux.ReleaseKind) (res []flux.ReleaseAction, err error) {
	s.helper.Log("method", "releaseAllForImage", "kind", kind)
	defer func() { s.helper.Log("method", "releaseAllForImage", "kind", kind, "res", len(res), "err", err) }()

	res = append(res, s.releaseActionNop(fmt.Sprintf("I'm going to release image %s to all services that would use it. Here we go.", target)))

	serviceIDs, err := s.helper.AllServices()
	if err != nil {
		return nil, errors.Wrap(err, "fetching all platform services")
	}

	containerMap, err := s.helper.AllImagesFor(serviceIDs)
	if err != nil {
		return nil, errors.Wrap(err, "fetching images for services")
	}

	// Each service is running multiple images.
	// Each image may need to be modified, and trigger a release.

	regradeMap := map[flux.ServiceID][]containerRegrade{}
	for serviceID, containers := range containerMap {
		for _, container := range containers {
			candidate := flux.ParseImageID(container.Image)
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
	res = append(res, s.releaseActionCommitAndPush(fmt.Sprintf("Release %s to all services", target)))
	for service := range regradeMap {
		res = append(res, s.releaseActionReleaseService(service))
	}

	if kind == flux.ReleaseKindExecute {
		if err := s.execute(res); err != nil {
			return res, err
		}
	}

	return res, nil
}

func (s *releaser) releaseOneToLatest(id flux.ServiceID, kind flux.ReleaseKind) (res []flux.ReleaseAction, err error) {
	s.helper.Log("method", "releaseOneToLatest", "kind", kind)
	defer func() { s.helper.Log("method", "releaseOneToLatest", "kind", kind, "res", len(res), "err", err) }()

	res = append(res, s.releaseActionNop(fmt.Sprintf("I'm going to release the latest images(s) for service %s. Here we go.", id)))

	namespace, service := id.Components()
	containers, err := s.helper.Platform.ContainersFor(namespace, service)
	if err != nil {
		return nil, errors.Wrapf(err, "fetching images for service %s", id)
	}

	// Each service is running multiple images.
	// Each image may need to be modified, and trigger a release.

	var regrades []containerRegrade
	for _, container := range containers {
		imageID := flux.ParseImageID(container.Image)
		imageRepo, err := s.helper.Registry.GetRepository(imageID.Repository())
		if err != nil {
			return nil, errors.Wrapf(err, "fetching repository for %s", imageID)
		}
		if len(imageRepo.Images) <= 0 {
			continue // strange
		}

		latestID := flux.ParseImageID(imageRepo.Images[0].String())
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
	res = append(res, s.releaseActionCommitAndPush(fmt.Sprintf("Release latest images to %s", id)))
	res = append(res, s.releaseActionReleaseService(id))

	if kind == flux.ReleaseKindExecute {
		if err := s.execute(res); err != nil {
			return res, err
		}
	}

	return res, nil
}

func (s *releaser) releaseOne(serviceID flux.ServiceID, target flux.ImageID, kind flux.ReleaseKind) (res []flux.ReleaseAction, err error) {
	s.helper.Log("method", "releaseOne", "kind", kind)
	defer func() { s.helper.Log("method", "releaseOne", "kind", kind, "res", len(res), "err", err) }()

	res = append(res, s.releaseActionNop(fmt.Sprintf("I'm going to release image %s to service %s.", target, serviceID)))

	namespace, service := serviceID.Components()
	containers, err := s.helper.Platform.ContainersFor(namespace, service)
	if err != nil {
		return nil, errors.Wrapf(err, "fetching images for service %s", serviceID)
	}

	// Each service is running multiple images.
	// Each image may need to be modified, and trigger a release.

	var regrades []containerRegrade
	for _, container := range containers {
		candidate := flux.ParseImageID(container.Image)
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
	res = append(res, s.releaseActionCommitAndPush(fmt.Sprintf("Release %s to %s", target, serviceID)))
	res = append(res, s.releaseActionReleaseService(serviceID))

	if kind == flux.ReleaseKindExecute {
		if err := s.execute(res); err != nil {
			return res, err
		}
	}

	return res, nil
}

// Release whatever is in the cloned configuration, without changing anything
func (s *releaser) releaseOneWithoutUpdate(serviceID flux.ServiceID, kind flux.ReleaseKind) (res []flux.ReleaseAction, err error) {
	s.helper.Log("method", "releaseOneWithoutUpdate", "kind", kind)
	defer func() { s.helper.Log("method", "releaseOneWithoutUpdate", "kind", kind, "res", len(res), "err", err) }()

	actions := []flux.ReleaseAction{
		s.releaseActionNop(fmt.Sprintf("I'm going to release service %s using the config from the git repo, without updating it", serviceID)),
		s.releaseActionClone(),
		s.releaseActionFindPodController(serviceID),
		s.releaseActionReleaseService(serviceID),
	}
	if kind == flux.ReleaseKindExecute {
		return actions, s.execute(actions)
	}
	return actions, nil
}

// Release whatever is in the cloned configuration, without changing anything
func (s *releaser) releaseAllWithoutUpdate(kind flux.ReleaseKind) (res []flux.ReleaseAction, err error) {
	serviceIDs, err := s.helper.AllServices()
	if err != nil {
		return nil, errors.Wrap(err, "fetching all platform services")
	}

	actions := []flux.ReleaseAction{
		s.releaseActionNop("I'm going to release all services using the config from the git repo, without updating it"),
		s.releaseActionClone(),
	}

	for _, service := range serviceIDs {
		actions = append(actions,
			s.releaseActionFindPodController(service),
			s.releaseActionReleaseService(service))
	}

	if kind == flux.ReleaseKindExecute {
		return actions, s.execute(actions)
	}
	return actions, nil
}

func (s *releaser) execute(actions []flux.ReleaseAction) error {
	rc := flux.NewReleaseContext()
	defer rc.Clean()

	for _, action := range actions {
		s.helper.Log("description", action.Description)
		if action.Do == nil {
			continue
		}

		if err := action.Do(rc); err != nil {
			s.helper.Log("err", err)
			return err
		}
	}

	return nil
}

// Release helpers.

type containerRegrade struct {
	container string
	current   flux.ImageID
	target    flux.ImageID
}

// ReleaseAction Do funcs

func (s *releaser) releaseActionNop(desc string) flux.ReleaseAction {
	return flux.ReleaseAction{Description: desc}
}

func (s *releaser) releaseActionClone() flux.ReleaseAction {
	return flux.ReleaseAction{
		Description: "Clone the config repo.",
		Do: func(rc *flux.ReleaseContext) error {
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

func (s *releaser) releaseActionFindPodController(service flux.ServiceID) flux.ReleaseAction {
	return flux.ReleaseAction{
		Description: fmt.Sprintf("Load the resource definition file for service %s", service),
		Do: func(rc *flux.ReleaseContext) error {
			if fi, err := os.Stat(rc.RepoPath); err != nil || !fi.IsDir() {
				return fmt.Errorf("the repo path (%s) is not valid", rc.RepoPath)
			}

			namespace, serviceName := service.Components()
			files, err := kubernetes.FilesFor(rc.RepoPath, namespace, serviceName)

			if err != nil {
				return errors.Wrapf(err, "finding resource definition file for %s", service)
			}
			if len(files) <= 0 {
				return fmt.Errorf("no resource definition file found for %s", service)
			}
			if len(files) > 1 {
				return fmt.Errorf("multiple resource definition files found for %s: %s", service, strings.Join(files, ", "))
			}

			def, err := ioutil.ReadFile(files[0]) // TODO(mb) not multi-doc safe
			if err != nil {
				return err
			}
			rc.PodControllers[service] = def
			return nil
		},
	}
}

func (s *releaser) releaseActionUpdatePodController(service flux.ServiceID, regrades []containerRegrade) flux.ReleaseAction {
	var actions []string
	for _, regrade := range regrades {
		actions = append(actions, fmt.Sprintf("%s (%s -> %s)", regrade.container, regrade.current, regrade.target))
	}
	actionList := strings.Join(actions, ", ")

	return flux.ReleaseAction{
		Description: fmt.Sprintf("Update %d images(s) in the resource definition file for %s: %s.", len(regrades), service, actionList),
		Do: func(rc *flux.ReleaseContext) error {
			resourcePath := filepath.Join(rc.RepoPath, s.repo.Path)
			if fi, err := os.Stat(resourcePath); err != nil || !fi.IsDir() {
				return fmt.Errorf("the resource path (%s) is not valid", resourcePath)
			}

			namespace, serviceName := service.Components()
			files, err := kubernetes.FilesFor(resourcePath, namespace, serviceName)
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

func (s *releaser) releaseActionCommitAndPush(msg string) flux.ReleaseAction {
	return flux.ReleaseAction{
		Description: "Commit and push the config repo.",
		Do: func(rc *flux.ReleaseContext) error {
			if fi, err := os.Stat(rc.RepoPath); err != nil || !fi.IsDir() {
				return fmt.Errorf("the repo path (%s) is not valid", rc.RepoPath)
			}
			if _, err := os.Stat(rc.RepoKey); err != nil {
				return fmt.Errorf("the repo key (%s) is not valid: %v", rc.RepoKey, err)
			}
			return s.repo.CommitAndPush(rc.RepoPath, rc.RepoKey, msg)
		},
	}
}

func (s *releaser) releaseActionReleaseService(service flux.ServiceID) flux.ReleaseAction {
	return flux.ReleaseAction{
		Description: fmt.Sprintf("Release the service %s.", service),
		Do: func(rc *flux.ReleaseContext) error {
			def, ok := rc.PodControllers[service]
			if !ok {
				return errors.New("didn't find pod controller definition for " + string(service))
			}
			namespace, serviceName := service.Components()
			return s.helper.Platform.Release(namespace, serviceName, def)
		},
	}
}
