package release

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
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
	WorkingDir     string
	KeyPath        string
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
	rc.WorkingDir = path
	rc.KeyPath = keyfile
	return nil
}

func (rc *ReleaseContext) CommitAndPush(msg string) (string, error) {
	return rc.Instance.ConfigRepo().CommitAndPush(rc.WorkingDir, rc.KeyPath, msg)
}

func (rc *ReleaseContext) ConfigPath() string {
	return filepath.Join(rc.WorkingDir, rc.Instance.ConfigRepo().Path)
}

func (rc *ReleaseContext) Clean() {
	if rc.WorkingDir != "" {
		os.RemoveAll(rc.WorkingDir)
	}
}

type serviceQuery func(*instance.Instance) ([]platform.Service, error)

func exactlyTheseServices(include []flux.ServiceID) serviceQuery {
	return func(h *instance.Instance) ([]platform.Service, error) {
		return h.GetServices(include)
	}
}

func allServicesExcept(exclude flux.ServiceIDSet) serviceQuery {
	return func(h *instance.Instance) ([]platform.Service, error) {
		return h.GetAllServicesExcept("", exclude)
	}
}

type imageCollect func(*instance.Instance, []platform.Service) (instance.ImageMap, error)

func allLatestImages(h *instance.Instance, services []platform.Service) (instance.ImageMap, error) {
	return h.CollectAvailableImages(services)
}

func exactlyTheseImages(images []flux.ImageID) imageCollect {
	return func(h *instance.Instance, _ []platform.Service) (instance.ImageMap, error) {
		return h.ExactImages(images)
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

	exclude := flux.ServiceIDSet{}
	exclude.Add(job.Spec.Excludes)

	locked, err := lockedServices(inst)
	if err != nil {
		return err
	}
	exclude.Add(locked)

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
		return r.releaseImages(releaseType, "Release latest images to all services", inst, job.Spec.Kind, allServicesExcept(exclude), allLatestImages, updateJob)

	case job.Spec.ServiceSpec == flux.ServiceSpecAll && job.Spec.ImageSpec == flux.ImageSpecNone:
		releaseType = "release_all_without_update"
		return r.releaseWithoutUpdate(releaseType, "Apply latest config to all services", inst, job.Spec.Kind, allServicesExcept(exclude), updateJob)

	case job.Spec.ServiceSpec == flux.ServiceSpecAll:
		releaseType = "release_all_for_image"
		imageID := flux.ParseImageID(string(job.Spec.ImageSpec))
		return r.releaseImages(releaseType, fmt.Sprintf("Release %s to all services", imageID), inst, job.Spec.Kind, allServicesExcept(exclude), exactlyTheseImages([]flux.ImageID{imageID}), updateJob)

	case job.Spec.ImageSpec == flux.ImageSpecLatest:
		releaseType = "release_one_to_latest"
		serviceID, err := flux.ParseServiceID(string(job.Spec.ServiceSpec))
		if err != nil {
			return errors.Wrapf(err, "parsing service ID from spec %s", job.Spec.ServiceSpec)
		}
		services := flux.ServiceIDs([]flux.ServiceID{serviceID}).Without(exclude)
		return r.releaseImages(releaseType, fmt.Sprintf("Release latest images to %s", serviceID), inst, job.Spec.Kind, exactlyTheseServices(services), allLatestImages, updateJob)

	case job.Spec.ImageSpec == flux.ImageSpecNone:
		releaseType = "release_one_without_update"
		serviceID, err := flux.ParseServiceID(string(job.Spec.ServiceSpec))
		if err != nil {
			return errors.Wrapf(err, "parsing service ID from spec %s", job.Spec.ServiceSpec)
		}
		services := flux.ServiceIDs([]flux.ServiceID{serviceID}).Without(exclude)
		return r.releaseWithoutUpdate(releaseType, fmt.Sprintf("Apply latest config to %s", serviceID), inst, job.Spec.Kind, exactlyTheseServices(services), updateJob)

	default:
		releaseType = "release_one"
		serviceID, err := flux.ParseServiceID(string(job.Spec.ServiceSpec))
		if err != nil {
			return errors.Wrapf(err, "parsing service ID from spec %s", job.Spec.ServiceSpec)
		}
		services := flux.ServiceIDs([]flux.ServiceID{serviceID}).Without(exclude)
		imageID := flux.ParseImageID(string(job.Spec.ImageSpec))
		return r.releaseImages(releaseType, fmt.Sprintf("Release %s to %s", imageID, serviceID), inst, job.Spec.Kind, exactlyTheseServices(services), exactlyTheseImages([]flux.ImageID{imageID}), updateJob)
	}
}

func (r *releaser) releaseImages(method, msg string, inst *instance.Instance, kind flux.ReleaseKind, getServices serviceQuery, getImages imageCollect, updateJob func(string, ...interface{})) (err error) {
	var res []ReleaseAction
	defer func() {
		if err == nil {
			err = r.execute(inst, res, kind, updateJob)
		}
	}()

	res = append(res, r.releaseActionPrintf(msg))

	var (
		base  = r.metrics.StageDuration.With("method", method)
		stage *metrics.Timer
	)

	defer func() { stage.ObserveDuration() }()
	stage = metrics.NewTimer(base.With("stage", "fetch_platform_services"))

	services, err := getServices(inst)
	if err != nil {
		return errors.Wrap(err, "fetching platform services")
	}

	stage.ObserveDuration()
	stage = metrics.NewTimer(base.With("stage", "calculate_regrades"))

	// Each service is running multiple images.
	// Each image may need to be upgraded, and trigger a release.
	images, err := getImages(inst, services)
	if err != nil {
		return errors.Wrap(err, "collecting available images to calculate regrades")
	}

	regradeMap := map[flux.ServiceID][]containerRegrade{}
	for _, service := range services {
		containers, err := service.ContainersOrError()
		if err != nil {
			res = append(res, r.releaseActionPrintf("service %s does not have images associated: %s", service.ID, err))
			continue
		}
		for _, container := range containers {
			currentImageID := flux.ParseImageID(container.Image)
			latestImage := images.LatestImage(currentImageID.Repository())
			if latestImage == nil {
				continue
			}

			if currentImageID == latestImage.ID {
				res = append(res, r.releaseActionPrintf("Service %s image %s is already the latest one; skipping.", service.ID, currentImageID))
				continue
			}

			regradeMap[service.ID] = append(regradeMap[service.ID], containerRegrade{
				container: container.Name,
				current:   currentImageID,
				target:    latestImage.ID,
			})
		}
	}

	if len(regradeMap) <= 0 {
		res = append(res, r.releaseActionPrintf("All selected services are running the requested images. Nothing to do."))
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
	res = append(res, r.releaseActionCommitAndPush(msg))
	var servicesToRegrade []flux.ServiceID
	for service := range regradeMap {
		servicesToRegrade = append(servicesToRegrade, service)
	}
	res = append(res, r.releaseActionRegradeServices(servicesToRegrade, msg))

	return nil
}

// Get set of all locked services
func lockedServices(inst *instance.Instance) ([]flux.ServiceID, error) {
	config, err := inst.GetConfig()
	if err != nil {
		return nil, err
	}

	ids := []flux.ServiceID{}
	for id, s := range config.Services {
		if s.Locked {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// Release whatever is in the cloned configuration, without changing anything
func (r *releaser) releaseWithoutUpdate(method, msg string, inst *instance.Instance, kind flux.ReleaseKind, getServices serviceQuery, updateJob func(string, ...interface{})) (err error) {
	var res []ReleaseAction
	defer func() {
		if err == nil {
			err = r.execute(inst, res, kind, updateJob)
		}
	}()

	var (
		base  = r.metrics.StageDuration.With("method", method)
		stage *metrics.Timer
	)

	defer func() { stage.ObserveDuration() }()
	stage = metrics.NewTimer(base.With("stage", "fetch_platform_services"))

	services, err := getServices(inst)
	if err != nil {
		return errors.Wrap(err, "fetching platform services")
	}

	stage.ObserveDuration()
	stage = metrics.NewTimer(base.With("stage", "finalize"))

	res = append(res, r.releaseActionPrintf(msg))
	res = append(res, r.releaseActionClone())

	ids := []flux.ServiceID{}
	for _, service := range services {
		res = append(res, r.releaseActionFindPodController(service.ID))
		ids = append(ids, service.ID)
	}
	res = append(res, r.releaseActionRegradeServices(ids, msg))

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

			if fi, err := os.Stat(rc.WorkingDir); err != nil || !fi.IsDir() {
				return "", fmt.Errorf("the repo path (%s) is not valid", rc.WorkingDir)
			}
			if _, err := os.Stat(rc.KeyPath); err != nil {
				return "", fmt.Errorf("the repo key (%s) is not valid: %v", rc.KeyPath, err)
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

func (r *releaser) releaseActionRegradeServices(services []flux.ServiceID, msg string) ReleaseAction {
	return ReleaseAction{
		Description: fmt.Sprintf("Regrade %d service(s): %s.", len(services), strings.Join(service2string(services), ", ")),
		Do: func(rc *ReleaseContext) (res string, err error) {
			defer func(begin time.Time) {
				r.metrics.ActionDuration.With(
					"action", "regrade_services",
					"success", fmt.Sprint(err == nil),
				).Observe(time.Since(begin).Seconds())
			}(time.Now())

			cause := strconv.Quote(msg)

			// We'll collect results for each service regrade.
			results := map[flux.ServiceID]error{}

			// Collect specs for each service regrade.
			var specs []platform.RegradeSpec
			for _, service := range services {
				def, ok := rc.PodControllers[service]
				if !ok {
					results[service] = errors.New("no definition found; skipping regrade")
					continue
				}

				namespace, serviceName := service.Components()
				rc.Instance.LogEvent(namespace, serviceName, "Starting regrade "+cause)
				specs = append(specs, platform.RegradeSpec{
					ServiceID:     service,
					NewDefinition: def,
				})
			}

			// Execute the regrades as a single transaction.
			// Splat any errors into our results map.
			transactionErr := rc.Instance.PlatformRegrade(specs)
			if transactionErr != nil {
				switch err := transactionErr.(type) {
				case platform.RegradeError:
					for id, regradeErr := range err {
						results[id] = regradeErr
					}
				default: // assume everything failed, if there was a coverall error
					for _, service := range services {
						results[service] = transactionErr
					}
				}
			}

			// Report individual service regrade results.
			for _, service := range services {
				namespace, serviceName := service.Components()
				if err := results[service]; err == nil { // no entry = nil error
					rc.Instance.LogEvent(namespace, serviceName, "Regrade due to "+cause+": done")
				} else {
					rc.Instance.LogEvent(namespace, serviceName, "Regrade due to "+cause+": failed: "+err.Error())
				}
			}

			return "", transactionErr
		},
	}
}
