package release

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/jobs"
	"github.com/weaveworks/flux/platform"
	"github.com/weaveworks/flux/platform/kubernetes"
)

const FluxServiceName = "fluxsvc"
const FluxDaemonName = "fluxd"

type Releaser struct {
	instancer instance.Instancer
	metrics   Metrics
}

type Metrics struct {
	ReleaseDuration metrics.Histogram
	ActionDuration  metrics.Histogram
	StageDuration   metrics.Histogram
}

func NewReleaser(
	instancer instance.Instancer,
	metrics Metrics,
) *Releaser {
	return &Releaser{
		instancer: instancer,
		metrics:   metrics,
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

func (r *Releaser) Handle(job *jobs.Job, updater jobs.JobUpdater) (err error) {
	params := job.Params.(jobs.ReleaseJobParams)
	releaseType := "unknown"
	defer func(begin time.Time) {
		r.metrics.ReleaseDuration.With(
			"release_type", releaseType,
			"release_kind", fmt.Sprint(params.Kind),
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	inst, err := r.instancer.Get(job.Instance)
	if err != nil {
		return err
	}

	inst.Logger = log.NewContext(inst.Logger).With("job", job.ID)

	updateJob := func(format string, args ...interface{}) {
		status := fmt.Sprintf(format, args...)
		job.Status = status
		job.Log = append(job.Log, status)
		updater.UpdateJob(*job)
	}

	exclude := flux.ServiceIDSet{}
	exclude.Add(params.Excludes)

	locked, err := lockedServices(inst)
	if err != nil {
		return err
	}
	exclude.Add(locked)

	updateJob("Calculating release actions.")

	switch {
	case params.ServiceSpec == flux.ServiceSpecAll && params.ImageSpec == flux.ImageSpecLatest:
		releaseType = "release_all_to_latest"
		return r.releaseImages(releaseType, "Release latest images to all services", inst, params.Kind, allServicesExcept(exclude), allLatestImages, updateJob)

	case params.ServiceSpec == flux.ServiceSpecAll && params.ImageSpec == flux.ImageSpecNone:
		releaseType = "release_all_without_update"
		return r.releaseWithoutUpdate(releaseType, "Apply latest config to all services", inst, params.Kind, allServicesExcept(exclude), updateJob)

	case params.ServiceSpec == flux.ServiceSpecAll:
		releaseType = "release_all_for_image"
		imageID := flux.ParseImageID(string(params.ImageSpec))
		return r.releaseImages(releaseType, fmt.Sprintf("Release %s to all services", imageID), inst, params.Kind, allServicesExcept(exclude), exactlyTheseImages([]flux.ImageID{imageID}), updateJob)

	case params.ImageSpec == flux.ImageSpecLatest:
		releaseType = "release_one_to_latest"
		serviceID, err := flux.ParseServiceID(string(params.ServiceSpec))
		if err != nil {
			return errors.Wrapf(err, "parsing service ID from params %s", params.ServiceSpec)
		}
		services := flux.ServiceIDs([]flux.ServiceID{serviceID}).Without(exclude)
		return r.releaseImages(releaseType, fmt.Sprintf("Release latest images to %s", serviceID), inst, params.Kind, exactlyTheseServices(services), allLatestImages, updateJob)

	case params.ImageSpec == flux.ImageSpecNone:
		releaseType = "release_one_without_update"
		serviceID, err := flux.ParseServiceID(string(params.ServiceSpec))
		if err != nil {
			return errors.Wrapf(err, "parsing service ID from params %s", params.ServiceSpec)
		}
		services := flux.ServiceIDs([]flux.ServiceID{serviceID}).Without(exclude)
		return r.releaseWithoutUpdate(releaseType, fmt.Sprintf("Apply latest config to %s", serviceID), inst, params.Kind, exactlyTheseServices(services), updateJob)

	default:
		releaseType = "release_one"
		serviceID, err := flux.ParseServiceID(string(params.ServiceSpec))
		if err != nil {
			return errors.Wrapf(err, "parsing service ID from params %s", params.ServiceSpec)
		}
		services := flux.ServiceIDs([]flux.ServiceID{serviceID}).Without(exclude)
		imageID := flux.ParseImageID(string(params.ImageSpec))
		return r.releaseImages(releaseType, fmt.Sprintf("Release %s to %s", imageID, serviceID), inst, params.Kind, exactlyTheseServices(services), exactlyTheseImages([]flux.ImageID{imageID}), updateJob)
	}
}

func (r *Releaser) releaseImages(method, msg string, inst *instance.Instance, kind flux.ReleaseKind, getServices serviceQuery, getImages imageCollect, updateJob func(string, ...interface{})) (err error) {
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
	stage = metrics.NewTimer(base.With("stage", "calculate_applies"))

	// Each service is running multiple images.
	// Each image may need to be upgraded, and trigger an apply.
	images, err := getImages(inst, services)
	if err != nil {
		return errors.Wrap(err, "collecting available images to calculate applies")
	}

	updateMap := map[flux.ServiceID][]containerUpdate{}
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

			updateMap[service.ID] = append(updateMap[service.ID], containerUpdate{
				container: container.Name,
				current:   currentImageID,
				target:    latestImage.ID,
			})
		}
	}

	if len(updateMap) <= 0 {
		res = append(res, r.releaseActionPrintf("All selected services are running the requested images. Nothing to do."))
		return nil
	}

	stage.ObserveDuration()
	stage = metrics.NewTimer(base.With("stage", "finalize"))

	// We have identified at least 1 release that needs to occur. Releasing
	// means cloning the repo, changing the resource file(s), committing and
	// pushing, and then making the release(s) to the platform.

	res = append(res, r.releaseActionClone())
	for service, applies := range updateMap {
		res = append(res, r.releaseActionUpdatePodController(service, applies))
	}
	res = append(res, r.releaseActionCommitAndPush(msg))
	var servicesToApply []flux.ServiceID
	for service := range updateMap {
		servicesToApply = append(servicesToApply, service)
	}
	res = append(res, r.releaseActionReleaseServices(servicesToApply, msg))

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
func (r *Releaser) releaseWithoutUpdate(method, msg string, inst *instance.Instance, kind flux.ReleaseKind, getServices serviceQuery, updateJob func(string, ...interface{})) (err error) {
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
	res = append(res, r.releaseActionReleaseServices(ids, msg))

	return nil
}

func (r *Releaser) execute(inst *instance.Instance, actions []ReleaseAction, kind flux.ReleaseKind, updateJob func(string, ...interface{})) error {
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

type containerUpdate struct {
	container string
	current   flux.ImageID
	target    flux.ImageID
}

// ReleaseAction Do funcs

func (r *Releaser) releaseActionPrintf(format string, args ...interface{}) ReleaseAction {
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

func (r *Releaser) releaseActionClone() ReleaseAction {
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

func (r *Releaser) releaseActionFindPodController(service flux.ServiceID) ReleaseAction {
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

func (r *Releaser) releaseActionUpdatePodController(service flux.ServiceID, updates []containerUpdate) ReleaseAction {
	var actions []string
	for _, update := range updates {
		actions = append(actions, fmt.Sprintf("%s (%s -> %s)", update.container, update.current, update.target))
	}
	actionList := strings.Join(actions, ", ")

	return ReleaseAction{
		Description: fmt.Sprintf("Update %d images(s) in the resource definition file for %s: %s.", len(updates), service, actionList),
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

			for _, update := range updates {
				// Note 1: UpdatePodController parses the target (new) image
				// name, extracts the repository, and only mutates the line(s)
				// in the definition that match it. So for the time being we
				// ignore the current image. UpdatePodController could be
				// updated, if necessary.
				//
				// Note 2: we keep overwriting the same def, to handle multiple
				// images in a single file.
				def, err = kubernetes.UpdatePodController(def, string(update.target), ioutil.Discard)
				if err != nil {
					return "", errors.Wrapf(err, "updating pod controller for %s", update.target)
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

func (r *Releaser) releaseActionCommitAndPush(msg string) ReleaseAction {
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

func (r *Releaser) releaseActionReleaseServices(services []flux.ServiceID, msg string) ReleaseAction {
	return ReleaseAction{
		Description: fmt.Sprintf("Release %d service(s): %s.", len(services), strings.Join(service2string(services), ", ")),
		Do: func(rc *ReleaseContext) (res string, err error) {
			defer func(begin time.Time) {
				r.metrics.ActionDuration.With(
					"action", "release_services",
					"success", fmt.Sprint(err == nil),
				).Observe(time.Since(begin).Seconds())
			}(time.Now())

			cause := strconv.Quote(msg)

			// We'll collect results for each service release.
			results := map[flux.ServiceID]error{}

			// Collect definitions for each service release.
			var defs []platform.ServiceDefinition
			// If we're regrading our own image, we want to do that
			// last, and "asynchronously" (meaning we probably won't
			// see the reply).
			var asyncDefs []platform.ServiceDefinition

			for _, service := range services {
				def, ok := rc.PodControllers[service]
				if !ok {
					results[service] = errors.New("no definition found; skipping release")
					continue
				}

				namespace, serviceName := service.Components()
				switch serviceName {
				case FluxServiceName, FluxDaemonName:
					rc.Instance.LogEvent(namespace, serviceName, "Starting "+cause+". (no result expected)")
					asyncDefs = append(asyncDefs, platform.ServiceDefinition{
						ServiceID:     service,
						NewDefinition: def,
					})
				default:
					rc.Instance.LogEvent(namespace, serviceName, "Starting "+cause)
					defs = append(defs, platform.ServiceDefinition{
						ServiceID:     service,
						NewDefinition: def,
					})
				}
			}

			// Execute the releases as a single transaction.
			// Splat any errors into our results map.
			transactionErr := rc.Instance.PlatformApply(defs)
			if transactionErr != nil {
				switch err := transactionErr.(type) {
				case platform.ApplyError:
					for id, applyErr := range err {
						results[id] = applyErr
					}
				default: // assume everything failed, if there was a coverall error
					for _, service := range services {
						results[service] = transactionErr
					}
				}
			}

			// Report individual service release results.
			for _, service := range services {
				namespace, serviceName := service.Components()
				switch serviceName {
				case FluxServiceName, FluxDaemonName:
					continue
				default:
					if err := results[service]; err == nil { // no entry = nil error
						rc.Instance.LogEvent(namespace, serviceName, msg+". done")
					} else {
						rc.Instance.LogEvent(namespace, serviceName, msg+". error: "+err.Error()+". failed")
					}
				}
			}

			// Lastly, services for which we don't expect a result
			// (i.e., ourselves). This will kick off the release in
			// the daemon, which will cause Kubernetes to restart the
			// service. In the meantime, however, we will have
			// finished recording what happened, as part of a graceful
			// shutdown. So the only thing that goes missing is the
			// result from this release call.
			if len(asyncDefs) > 0 {
				go func() {
					rc.Instance.PlatformApply(asyncDefs)
				}()
			}

			return "", transactionErr
		},
	}
}
