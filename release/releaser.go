package release

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-kit/kit/metrics"
	"github.com/pkg/errors"

	"github.com/weaveworks/fluxy"
	"github.com/weaveworks/fluxy/instance"
	"github.com/weaveworks/fluxy/platform"
	"github.com/weaveworks/fluxy/platform/kubernetes"
)

type releaser struct {
	instancer instance.Instancer
	metrics   Metrics
	semaphore chan struct{}
}

const maxSimultaneousReleases = 1

type Metrics struct {
	ReleaseDuration metrics.Histogram
	ActionDuration  metrics.Histogram
	StageDuration   metrics.Histogram
}

type ServiceIDSet map[flux.ServiceID]struct{}

func (s ServiceIDSet) Add(ids []flux.ServiceID) {
	for _, id := range ids {
		s[id] = struct{}{}
	}
}

func (s ServiceIDSet) Contains(id flux.ServiceID) bool {
	_, ok := s[id]
	return ok
}

type ServiceIDs []flux.ServiceID

func (ids ServiceIDs) Without(set ServiceIDSet) (res ServiceIDs) {
	for _, id := range ids {
		if !set.Contains(id) {
			res = append(res, id)
		}
	}
	return res
}

func newReleaser(
	instancer instance.Instancer,
	metrics Metrics,
) *releaser {
	return &releaser{
		instancer: instancer,
		metrics:   metrics,
		semaphore: make(chan struct{}, maxSimultaneousReleases),
	}
}

type ReleaseAction struct {
	Description string                                `json:"description"`
	Do          func(*ReleaseContext) (string, error) `json:"-"`
	Result      string                                `json:"result"`
}

type ReleaseContext struct {
	Instance       *instance.Instance
	RepoPath       string
	RepoKey        string
	PodControllers map[flux.ServiceID][]byte
}

func NewReleaseContext(inst *instance.Instance) *ReleaseContext {
	return &ReleaseContext{
		Instance:       inst,
		PodControllers: map[flux.ServiceID][]byte{},
	}
}

func (rc *ReleaseContext) CloneConfig() error {
	path, keyfile, err := rc.Instance.ConfigRepo().Clone()
	if err != nil {
		return err
	}
	rc.RepoPath = path
	rc.RepoKey = keyfile
	return nil
}

func (rc *ReleaseContext) CommitAndPush(msg string) (string, error) {
	return rc.Instance.ConfigRepo().CommitAndPush(rc.RepoPath, rc.RepoKey, msg)
}

func (rc *ReleaseContext) ConfigPath() string {
	return filepath.Join(rc.RepoPath, rc.Instance.ConfigRepo().Path)
}

func (rc *ReleaseContext) Clean() {
	if rc.RepoPath != "" {
		os.RemoveAll(rc.RepoPath)
	}
}

func (r *releaser) Release(job *flux.ReleaseJob, updater flux.ReleaseJobUpdater) (err error) {
	releaseType := "unknown"
	defer func(begin time.Time) {
		r.metrics.ReleaseDuration.With(
			"release_type", releaseType,
			"release_kind", fmt.Sprint(job.Spec.Kind),
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	inst, err := r.instancer.Get(job.Instance)
	if err != nil {
		return err
	}

	updateJob := func(format string, args ...interface{}) {
		status := fmt.Sprintf(format, args...)
		job.Status = status
		job.Log = append(job.Log, status)
		updater.UpdateJob(*job)
	}

	exclude := ServiceIDSet{}
	exclude.Add(job.Spec.Excludes)

	select {
	case r.semaphore <- struct{}{}:
		break // we've acquired the lock
	default:
		return errors.New("a release is already in progress; please try again later")
	}
	defer func() { <-r.semaphore }()

	updateJob("Calculating release actions.")

	switch {
	case job.Spec.ServiceSpec == flux.ServiceSpecAll && job.Spec.ImageSpec == flux.ImageSpecLatest:
		releaseType = "release_all_to_latest"
		return r.releaseAllToLatest(inst, job.Spec.Kind, exclude, updateJob)

	case job.Spec.ServiceSpec == flux.ServiceSpecAll && job.Spec.ImageSpec == flux.ImageSpecNone:
		releaseType = "release_all_without_update"
		return r.releaseAllWithoutUpdate(inst, job.Spec.Kind, exclude, updateJob)

	case job.Spec.ServiceSpec == flux.ServiceSpecAll:
		releaseType = "release_all_for_image"
		imageID := flux.ParseImageID(string(job.Spec.ImageSpec))
		return r.releaseAllForImage(inst, imageID, job.Spec.Kind, exclude, updateJob)

	case job.Spec.ImageSpec == flux.ImageSpecLatest:
		releaseType = "release_one_to_latest"
		serviceID, err := flux.ParseServiceID(string(job.Spec.ServiceSpec))
		if err != nil {
			return errors.Wrapf(err, "parsing service ID from spec %s", job.Spec.ServiceSpec)
		}
		return r.releaseOneToLatest(inst, serviceID, job.Spec.Kind, exclude, updateJob)

	case job.Spec.ImageSpec == flux.ImageSpecNone:
		releaseType = "release_one_without_update"
		serviceID, err := flux.ParseServiceID(string(job.Spec.ServiceSpec))
		if err != nil {
			return errors.Wrapf(err, "parsing service ID from spec %s", job.Spec.ServiceSpec)
		}
		return r.releaseOneWithoutUpdate(inst, serviceID, job.Spec.Kind, exclude, updateJob)

	default:
		releaseType = "release_one"
		serviceID, err := flux.ParseServiceID(string(job.Spec.ServiceSpec))
		if err != nil {
			return errors.Wrapf(err, "parsing service ID from spec %s", job.Spec.ServiceSpec)
		}
		imageID := flux.ParseImageID(string(job.Spec.ImageSpec))
		return r.releaseOne(inst, serviceID, imageID, job.Spec.Kind, exclude, updateJob)
	}
}

func (r *releaser) releaseAllToLatest(inst *instance.Instance, kind flux.ReleaseKind, exclude ServiceIDSet, updateJob func(string, ...interface{})) (err error) {
	var res []ReleaseAction
	defer func() {
		if err == nil {
			err = r.execute(inst, res, kind, updateJob)
		}
	}()

	res = append(res, r.releaseActionPrintf("I'm going to release all services to their latest images."))

	var (
		base  = r.metrics.StageDuration.With("method", "release_all_to_latest")
		stage *metrics.Timer
	)

	defer func() { stage.ObserveDuration() }()
	stage = metrics.NewTimer(base.With("stage", "fetch_all_platform_services"))

	serviceIDs, err := inst.AllServices()
	if err != nil {
		return errors.Wrap(err, "fetching all platform services")
	}

	stage.ObserveDuration()
	stage = metrics.NewTimer(base.With("stage", "all_releasable_images_for"))

	locked, err := lockedServices(inst)
	if err != nil {
		return err
	}
	containerMap, err := inst.AllReleasableImagesFor(ServiceIDs(serviceIDs).Without(exclude).Without(locked))
	if err != nil {
		return errors.Wrap(err, "fetching images for services")
	}

	stage.ObserveDuration()
	stage = metrics.NewTimer(base.With("stage", "calculate_regrades"))

	// Each service is running multiple images.
	// Each image may need to be upgraded, and trigger a release.

	regradeMap := map[flux.ServiceID][]containerRegrade{}
	for serviceID, containers := range containerMap {
		for _, container := range containers {
			currentImageID := flux.ParseImageID(container.Image)
			imageRepo, err := inst.RegistryGetRepository(currentImageID.Repository())
			if err != nil {
				return errors.Wrapf(err, "fetching image repo for %s", currentImageID)
			}
			latestImage, err := imageRepo.LatestImage()
			if err != nil {
				return errors.Wrapf(err, "getting latest image from %s", imageRepo.Name)
			}
			latestImageID := flux.ParseImageID(latestImage.String())
			if currentImageID == latestImageID {
				res = append(res, r.releaseActionPrintf("Service image %s is already the latest one; skipping.", currentImageID))
				continue
			}
			regradeMap[serviceID] = append(regradeMap[serviceID], containerRegrade{
				container: container.Name,
				current:   currentImageID,
				target:    latestImageID,
			})
		}
	}
	if len(regradeMap) <= 0 {
		res = append(res, r.releaseActionPrintf("All services are running the latest images. Nothing to do."))
		return nil
	}

	stage.ObserveDuration()
	stage = metrics.NewTimer(base.With("stage", "finalize"))

	// We have identified at least 1 release that needs to occur. Releasing
	// means cloning the repo, changing the resource file(s), committing and
	// pushing, and then making the release(s) to the platform.

	res = append(res, r.releaseActionClone())
	for service, regrades := range regradeMap {
		res = append(res, r.releaseActionUpdatePodController(service, regrades))
	}
	res = append(res, r.releaseActionCommitAndPush("Release latest images to all services"))
	var services []flux.ServiceID
	for service := range regradeMap {
		services = append(services, service)
	}
	res = append(res, r.releaseActionRegradeServices(services, "latest images (to all services)"))

	return nil
}

func (r *releaser) releaseAllForImage(inst *instance.Instance, target flux.ImageID, kind flux.ReleaseKind, exclude ServiceIDSet, updateJob func(string, ...interface{})) (err error) {
	var res []ReleaseAction
	defer func() {
		if err == nil {
			err = r.execute(inst, res, kind, updateJob)
		}
	}()

	res = append(res, r.releaseActionPrintf("I'm going to release image %s to all services that would use it.", target))

	var (
		base  = r.metrics.StageDuration.With("method", "release_all_for_image")
		stage *metrics.Timer
	)

	defer func() { stage.ObserveDuration() }()
	stage = metrics.NewTimer(base.With("stage", "fetch_all_platform_services"))

	serviceIDs, err := inst.AllServices()
	if err != nil {
		return errors.Wrap(err, "fetching all platform services")
	}

	stage.ObserveDuration()
	stage = metrics.NewTimer(base.With("stage", "all_releasable_images_for"))

	locked, err := lockedServices(inst)
	if err != nil {
		return err
	}
	containerMap, err := inst.AllReleasableImagesFor(ServiceIDs(serviceIDs).Without(exclude).Without(locked))
	if err != nil {
		return errors.Wrap(err, "fetching images for services")
	}

	// Each service is running multiple images.
	// Each image may need to be modified, and trigger a release.

	stage.ObserveDuration()
	stage = metrics.NewTimer(base.With("stage", "calculate_regrades"))

	regradeMap := map[flux.ServiceID][]containerRegrade{}
	for serviceID, containers := range containerMap {
		for _, container := range containers {
			candidate := flux.ParseImageID(container.Image)
			if candidate.Repository() != target.Repository() {
				continue
			}
			if candidate == target {
				res = append(res, r.releaseActionPrintf("Service %s image %s matches the target image exactly. Skipping.", serviceID, candidate))
				continue
			}
			regradeMap[serviceID] = append(regradeMap[serviceID], containerRegrade{
				container: container.Name,
				current:   candidate,
				target:    target,
			})
		}
	}
	if len(regradeMap) <= 0 {
		res = append(res, r.releaseActionPrintf("All matching services are already running image %s. Nothing to do.", target))
		return nil
	}

	stage.ObserveDuration()
	stage = metrics.NewTimer(base.With("stage", "finalize"))

	// We have identified at least 1 release that needs to occur. Releasing
	// means cloning the repo, changing the resource file(s), committing and
	// pushing, and then making the release(s) to the platform.

	res = append(res, r.releaseActionClone())
	for service, imageReleases := range regradeMap {
		res = append(res, r.releaseActionUpdatePodController(service, imageReleases))
	}
	res = append(res, r.releaseActionCommitAndPush(fmt.Sprintf("Release %s to all services", target)))
	var services []flux.ServiceID
	for service := range regradeMap {
		services = append(services, service)
	}
	res = append(res, r.releaseActionRegradeServices(services, string(target)+" (to all services)"))

	return nil
}

func (r *releaser) releaseOneToLatest(inst *instance.Instance, id flux.ServiceID, kind flux.ReleaseKind, exclude ServiceIDSet, updateJob func(string, ...interface{})) (err error) {
	var res []ReleaseAction
	defer func() {
		if err == nil {
			err = r.execute(inst, res, kind, updateJob)
		}
	}()

	res = append(res, r.releaseActionPrintf("I'm going to release the latest images(s) for service %s.", id))

	var (
		base  = r.metrics.StageDuration.With("method", "release_one_to_latest")
		stage *metrics.Timer
	)

	defer func() { stage.ObserveDuration() }()
	stage = metrics.NewTimer(base.With("stage", "fetch_images_for_service"))

	if exclude.Contains(id) {
		res = append(res, r.releaseActionPrintf("Specified service %s is excluded; ignoring.", id))
		return nil
	}
	locked, err := lockedServices(inst)
	if err != nil {
		return err
	}
	if locked.Contains(id) {
		res = append(res, r.releaseActionPrintf("Specified service %s is locked; ignoring.", id))
		return nil
	}

	namespace, service := id.Components()
	containers, err := inst.PlatformContainersFor(namespace, service)
	if err != nil {
		return errors.Wrapf(err, "fetching images for service %s", id)
	}

	stage.ObserveDuration()
	stage = metrics.NewTimer(base.With("stage", "calculate_regrades"))

	// Each service is running multiple images.
	// Each image may need to be modified, and trigger a release.

	var regrades []containerRegrade
	for _, container := range containers {
		imageID := flux.ParseImageID(container.Image)
		imageRepo, err := inst.RegistryGetRepository(imageID.Repository())
		if err != nil {
			return errors.Wrapf(err, "fetching repository for %s", imageID)
		}
		if len(imageRepo.Images) <= 0 {
			res = append(res, r.releaseActionPrintf("The service image %s had no images available in its repository; very strange!", imageID))
			continue
		}

		latestImage, err := imageRepo.LatestImage()
		if err != nil {
			return errors.Wrapf(err, "getting latest image from %s", imageRepo.Name)
		}
		latestID := flux.ParseImageID(latestImage.String())
		if imageID == latestID {
			res = append(res, r.releaseActionPrintf("The service image %s is already at latest; skipping.", imageID))
			continue
		}
		regrades = append(regrades, containerRegrade{
			container: container.Name,
			current:   imageID,
			target:    latestID,
		})
	}
	if len(regrades) <= 0 {
		res = append(res, r.releaseActionPrintf("The service is already running the latest version of all its images. Nothing to do."))
		return nil
	}

	stage.ObserveDuration()
	stage = metrics.NewTimer(base.With("stage", "finalize"))

	// We need to make 1 release. Releasing means cloning the repo, changing the
	// resource file(s), committing and pushing, and then making the release(s)
	// to the platform.

	res = append(res, r.releaseActionClone())
	res = append(res, r.releaseActionUpdatePodController(id, regrades))
	res = append(res, r.releaseActionCommitAndPush(fmt.Sprintf("Release latest images to %s", id)))
	res = append(res, r.releaseActionRegradeServices([]flux.ServiceID{id}, "latest images"))

	return nil
}

func (r *releaser) releaseOne(inst *instance.Instance, serviceID flux.ServiceID, target flux.ImageID, kind flux.ReleaseKind, exclude ServiceIDSet, updateJob func(string, ...interface{})) (err error) {
	var res []ReleaseAction
	defer func() {
		if err == nil {
			err = r.execute(inst, res, kind, updateJob)
		}
	}()

	res = append(res, r.releaseActionPrintf("I'm going to release image %s to service %s.", target, serviceID))

	var (
		base  = r.metrics.StageDuration.With("method", "release_one")
		stage *metrics.Timer
	)

	defer func() { stage.ObserveDuration() }()
	stage = metrics.NewTimer(base.With("stage", "fetch_images_for_service"))

	if exclude.Contains(serviceID) {
		res = append(res, r.releaseActionPrintf("Specified service %s is excluded; ignoring.", serviceID))
		return nil
	}
	locked, err := lockedServices(inst)
	if err != nil {
		return err
	}
	if locked.Contains(serviceID) {
		res = append(res, r.releaseActionPrintf("Specified service %s is locked; ignoring.", serviceID))
		return nil
	}

	namespace, service := serviceID.Components()
	containers, err := inst.PlatformContainersFor(namespace, service)
	if err != nil {
		return errors.Wrapf(err, "fetching images for service %s", serviceID)
	}

	stage.ObserveDuration()
	stage = metrics.NewTimer(base.With("stage", "calculate_regrades"))

	// Each service is running multiple images.
	// Each image may need to be modified, and trigger a release.

	var regrades []containerRegrade
	for _, container := range containers {
		candidate := flux.ParseImageID(container.Image)
		if candidate.Repository() != target.Repository() {
			res = append(res, r.releaseActionPrintf("Image %s is different than %s. Ignoring.", candidate, target))
			continue
		}
		if candidate == target {
			res = append(res, r.releaseActionPrintf("Image %s is already released. Skipping.", candidate))
			continue
		}
		regrades = append(regrades, containerRegrade{
			container: container.Name,
			current:   candidate,
			target:    target,
		})
	}
	if len(regrades) <= 0 {
		res = append(res, r.releaseActionPrintf("Service %s doesn't need a regrade to %s. Nothing to do.", serviceID, target))
		return nil
	}

	stage.ObserveDuration()
	stage = metrics.NewTimer(base.With("stage", "finalize"))

	// We have identified at least 1 regrade that needs to occur. Releasing
	// means cloning the repo, changing the resource file(s), committing and
	// pushing, and then making the release(s) to the platform.

	res = append(res, r.releaseActionClone())
	res = append(res, r.releaseActionUpdatePodController(serviceID, regrades))
	res = append(res, r.releaseActionCommitAndPush(fmt.Sprintf("Release %s to %s", target, serviceID)))
	res = append(res, r.releaseActionRegradeServices([]flux.ServiceID{serviceID}, string(target)))

	return nil
}

// Release whatever is in the cloned configuration, without changing anything
func (r *releaser) releaseOneWithoutUpdate(inst *instance.Instance, serviceID flux.ServiceID, kind flux.ReleaseKind, exclude ServiceIDSet, updateJob func(string, ...interface{})) (err error) {
	var res []ReleaseAction
	defer func() {
		if err == nil {
			err = r.execute(inst, res, kind, updateJob)
		}
	}()

	var (
		base  = r.metrics.StageDuration.With("method", "release_one_without_update")
		stage *metrics.Timer
	)

	defer func() { stage.ObserveDuration() }()
	stage = metrics.NewTimer(base.With("stage", "finalize"))

	if exclude.Contains(serviceID) {
		res = append(res, r.releaseActionPrintf("Specified service %s is excluded; ignoring.", serviceID))
		return nil
	}
	locked, err := lockedServices(inst)
	if err != nil {
		return err
	}
	if locked.Contains(serviceID) {
		res = append(res, r.releaseActionPrintf("Specified service %s is locked; ignoring.", serviceID))
		return nil
	}

	res = append(res, r.releaseActionPrintf("I'm going to release service %s using the config from the git repo, without updating it", serviceID))
	res = append(res, r.releaseActionClone())
	res = append(res, r.releaseActionFindPodController(serviceID))
	res = append(res, r.releaseActionRegradeServices([]flux.ServiceID{serviceID}, "without update"))

	return nil
}

// Get set of all locked services
func lockedServices(inst *instance.Instance) (ServiceIDSet, error) {
	config, err := inst.GetConfig()
	if err != nil {
		return nil, err
	}

	locked := ServiceIDSet{}
	for id, s := range config.Services {
		if s.Locked {
			locked.Add([]flux.ServiceID{id})
		}
	}
	return locked, nil
}

// Release whatever is in the cloned configuration, without changing anything
func (r *releaser) releaseAllWithoutUpdate(inst *instance.Instance, kind flux.ReleaseKind, exclude ServiceIDSet, updateJob func(string, ...interface{})) (err error) {
	var res []ReleaseAction
	defer func() {
		if err == nil {
			err = r.execute(inst, res, kind, updateJob)
		}
	}()

	var (
		base  = r.metrics.StageDuration.With("method", "release_all_without_update")
		stage *metrics.Timer
	)

	defer func() { stage.ObserveDuration() }()
	stage = metrics.NewTimer(base.With("stage", "fetch_all_platform_services"))

	serviceIDs, err := inst.AllServices()
	if err != nil {
		return errors.Wrap(err, "fetching all platform services")
	}

	locked, err := lockedServices(inst)
	if err != nil {
		return err
	}
	serviceIDs = ServiceIDs(serviceIDs).Without(exclude).Without(locked)

	stage.ObserveDuration()
	stage = metrics.NewTimer(base.With("stage", "finalize"))

	res = append(res, r.releaseActionPrintf("I'm going to release all services using the config from the git repo, without updating it."))
	res = append(res, r.releaseActionClone())
	for _, service := range serviceIDs {
		res = append(res, r.releaseActionFindPodController(service))
	}
	res = append(res, r.releaseActionRegradeServices(serviceIDs, "without update (all services)"))

	return nil
}

func (r *releaser) execute(inst *instance.Instance, actions []ReleaseAction, kind flux.ReleaseKind, updateJob func(string, ...interface{})) error {
	rc := NewReleaseContext(inst)
	defer rc.Clean()

	for i, action := range actions {
		updateJob(action.Description)
		inst.Log("description", action.Description)
		if action.Do == nil {
			continue
		}

		if kind == flux.ReleaseKindExecute {
			result, err := action.Do(rc)
			if err != nil {
				updateJob(err.Error())
				inst.Log("err", err)
				actions[i].Result = "Failed: " + err.Error()
				return err
			}
			if result != "" {
				updateJob(result)
			}
			actions[i].Result = result
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

func (r *releaser) releaseActionPrintf(format string, args ...interface{}) ReleaseAction {
	return ReleaseAction{
		Description: fmt.Sprintf(format, args...),
		Do: func(_ *ReleaseContext) (res string, err error) {
			defer func(begin time.Time) {
				r.metrics.ActionDuration.With(
					"action", "printf",
					"success", fmt.Sprint(err == nil),
				).Observe(time.Since(begin).Seconds())
			}(time.Now())

			return "", nil
		},
	}
}

func (r *releaser) releaseActionClone() ReleaseAction {
	return ReleaseAction{
		Description: "Clone the config repo.",
		Do: func(rc *ReleaseContext) (res string, err error) {
			defer func(begin time.Time) {
				r.metrics.ActionDuration.With(
					"action", "clone",
					"success", fmt.Sprint(err == nil),
				).Observe(time.Since(begin).Seconds())
			}(time.Now())

			err = rc.CloneConfig()
			if err != nil {
				return "", errors.Wrap(err, "clone the config repo")
			}
			return "Clone OK.", nil
		},
	}
}

func (r *releaser) releaseActionFindPodController(service flux.ServiceID) ReleaseAction {
	return ReleaseAction{
		Description: fmt.Sprintf("Load the resource definition file for service %s", service),
		Do: func(rc *ReleaseContext) (res string, err error) {
			defer func(begin time.Time) {
				r.metrics.ActionDuration.With(
					"action", "find_pod_controller",
					"success", fmt.Sprint(err == nil),
				).Observe(time.Since(begin).Seconds())
			}(time.Now())

			resourcePath := rc.ConfigPath()
			if fi, err := os.Stat(resourcePath); err != nil || !fi.IsDir() {
				return "", fmt.Errorf("the resource path (%s) is not valid", resourcePath)
			}

			namespace, serviceName := service.Components()
			files, err := kubernetes.FilesFor(resourcePath, namespace, serviceName)

			if err != nil {
				return "", errors.Wrapf(err, "finding resource definition file for %s", service)
			}
			if len(files) <= 0 { // fine; we'll just skip it
				return fmt.Sprintf("no resource definition file found for %s; skipping", service), nil
			}
			if len(files) > 1 {
				return "", fmt.Errorf("multiple resource definition files found for %s: %s", service, strings.Join(files, ", "))
			}

			def, err := ioutil.ReadFile(files[0]) // TODO(mb) not multi-doc safe
			if err != nil {
				return "", err
			}
			rc.PodControllers[service] = def
			return "Found pod controller OK.", nil
		},
	}
}

func (r *releaser) releaseActionUpdatePodController(service flux.ServiceID, regrades []containerRegrade) ReleaseAction {
	var actions []string
	for _, regrade := range regrades {
		actions = append(actions, fmt.Sprintf("%s (%s -> %s)", regrade.container, regrade.current, regrade.target))
	}
	actionList := strings.Join(actions, ", ")

	return ReleaseAction{
		Description: fmt.Sprintf("Update %d images(s) in the resource definition file for %s: %s.", len(regrades), service, actionList),
		Do: func(rc *ReleaseContext) (res string, err error) {
			defer func(begin time.Time) {
				r.metrics.ActionDuration.With(
					"action", "update_pod_controller",
					"success", fmt.Sprint(err == nil),
				).Observe(time.Since(begin).Seconds())
			}(time.Now())

			resourcePath := rc.ConfigPath()
			if fi, err := os.Stat(resourcePath); err != nil || !fi.IsDir() {
				return "", fmt.Errorf("the resource path (%s) is not valid", resourcePath)
			}

			namespace, serviceName := service.Components()
			files, err := kubernetes.FilesFor(resourcePath, namespace, serviceName)
			if err != nil {
				return "", errors.Wrapf(err, "finding resource definition file for %s", service)
			}
			if len(files) <= 0 {
				return fmt.Sprintf("no resource definition file found for %s; skipping", service), nil
			}
			if len(files) > 1 {
				return "", fmt.Errorf("multiple resource definition files found for %s: %s", service, strings.Join(files, ", "))
			}

			def, err := ioutil.ReadFile(files[0])
			if err != nil {
				return "", err
			}
			fi, err := os.Stat(files[0])
			if err != nil {
				return "", err
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
					return "", errors.Wrapf(err, "updating pod controller for %s", regrade.target)
				}
			}

			// Write the file back, so commit/push works.
			if err := ioutil.WriteFile(files[0], def, fi.Mode()); err != nil {
				return "", err
			}

			// Put the def in the map, so release works.
			rc.PodControllers[service] = def
			return "Update pod controller OK.", nil
		},
	}
}

func (r *releaser) releaseActionCommitAndPush(msg string) ReleaseAction {
	return ReleaseAction{
		Description: "Commit and push the config repo.",
		Do: func(rc *ReleaseContext) (res string, err error) {
			defer func(begin time.Time) {
				r.metrics.ActionDuration.With(
					"action", "commit_and_push",
					"success", fmt.Sprint(err == nil),
				).Observe(time.Since(begin).Seconds())
			}(time.Now())

			if fi, err := os.Stat(rc.RepoPath); err != nil || !fi.IsDir() {
				return "", fmt.Errorf("the repo path (%s) is not valid", rc.RepoPath)
			}
			if _, err := os.Stat(rc.RepoKey); err != nil {
				return "", fmt.Errorf("the repo key (%s) is not valid: %v", rc.RepoKey, err)
			}
			result, err := rc.CommitAndPush(msg)
			if err == nil && result == "" {
				return "Pushed commit: " + msg, nil
			}
			return result, err
		},
	}
}

func service2string(a []flux.ServiceID) []string {
	s := make([]string, len(a))
	for i := range a {
		s[i] = string(a[i])
	}
	return s
}

func (r *releaser) releaseActionRegradeServices(services []flux.ServiceID, cause string) ReleaseAction {
	return ReleaseAction{
		Description: fmt.Sprintf("Regrade %d service(s): %s.", len(services), strings.Join(service2string(services), ", ")),
		Do: func(rc *ReleaseContext) (res string, err error) {
			defer func(begin time.Time) {
				r.metrics.ActionDuration.With(
					"action", "regrade_services",
					"success", fmt.Sprint(err == nil),
				).Observe(time.Since(begin).Seconds())
			}(time.Now())

			// We'll collect results for each service regrade.
			results := map[flux.ServiceID]error{}

			// Collect specs for each service regrade.
			var specs []platform.RegradeSpec
			for _, service := range services {
				def, ok := rc.PodControllers[service]
				if !ok {
					results[service] = errors.New("no pod controller in release context; skipping regrade")
					continue
				}

				namespace, serviceName := service.Components()
				rc.Instance.LogEvent(namespace, serviceName, "Starting regrade "+cause)
				specs = append(specs, platform.RegradeSpec{
					NamespacedService: platform.NamespacedService{
						Namespace: namespace,
						Service:   serviceName,
					},
					NewDefinition: def,
				})
			}

			// Execute the regrades as a single transaction.
			// Splat any errors into our results map.
			transactionErr := rc.Instance.PlatformRegrade(specs)
			if transactionErr != nil {
				for ns, regradeErr := range transactionErr.(platform.RegradeError) {
					id, _ := flux.ParseServiceID(fmt.Sprintf("%s/%s", ns.Namespace, ns.Service))
					results[id] = regradeErr
				}
			}

			// Report individual service regrade results.
			for _, service := range services {
				namespace, serviceName := service.Components()
				if err := results[service]; err == nil { // no entry = nil error
					rc.Instance.LogEvent(namespace, serviceName, "Regrade "+cause+": done")
				} else {
					rc.Instance.LogEvent(namespace, serviceName, "Regrade "+cause+": failed: "+err.Error())
				}
			}

			return "", transactionErr
		},
	}
}
