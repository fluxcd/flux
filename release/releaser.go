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
	Description string                      `json:"description"`
	Do          func(*Repo) (string, error) `json:"-"`
	Result      string                      `json:"result"`
}

func (r *Releaser) Handle(job *jobs.Job, updater jobs.JobUpdater) (followUps []jobs.Job, err error) {
	params := job.Params.(jobs.ReleaseJobParams)
	metric := "unknown"
	defer func(begin time.Time) {
		r.metrics.ReleaseDuration.With(
			"method", metric,
			"release_kind", fmt.Sprint(params.Kind),
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	inst, err := r.instancer.Get(job.Instance)
	if err != nil {
		return nil, err
	}

	inst.Logger = log.NewContext(inst.Logger).With("job", job.ID)

	updateJob := func(format string, args ...interface{}) {
		status := fmt.Sprintf(format, args...)
		job.Status = status
		job.Log = append(job.Log, status)
		updater.UpdateJob(*job)
	}

	updateJob("Calculating release actions.")

	services, err := serviceSelector(inst, params.ServiceSpecs, params.Excludes)
	if err != nil {
		return nil, err
	}

	images := imageSelectorForSpec(params.ImageSpec)

	switch {
	case params.ServiceSpec == flux.ServiceSpecAll && params.ImageSpec == flux.ImageSpecLatest:
		metric = "release_all_to_latest"
	case params.ServiceSpec == flux.ServiceSpecAll && params.ImageSpec == flux.ImageSpecNone:
		metric = "release_all_without_update"
	case params.ServiceSpec == flux.ServiceSpecAll:
		metric = "release_all_for_image"
	case params.ImageSpec == flux.ImageSpecLatest:
		metric = "release_one_to_latest"
	case params.ImageSpec == flux.ImageSpecNone:
		metric = "release_one_without_update"
	default:
		metric = "release_one"
	}

	message := fmt.Sprintf("Release %s to %s", images, services)
	return nil, r.releaseImages(metric, message, inst, params.Kind, services, images, updateJob)
}

func (r *Releaser) releaseImages(method, msg string, inst *instance.Instance, kind flux.ReleaseKind, getServices serviceQuery, getImages imageSelector, updateJob func(string, ...interface{})) (err error) {
	var res []ReleaseAction
	defer func() {
		if err == nil {
			err = r.execute(inst, res, kind, updateJob)
		}
	}()

	var (
		metric = r.metrics.StageDuration.With("method", method)
	)

	res = append(res, r.releaseActionPrintf(msg))

	repo, images, err := r.gatherMaterials(metric, inst, getImages)
	if err != nil {
		return err
	}

	changed, err := r.updateDefinitions(metric, repo, getServices, images, kind)
	if err != nil {
		return err
	}

	return r.applyNewDefinitions(metric, inst, changed, kind)
}

func (r *Releaser) releaseImages(method, msg string, inst *instance.Instance, kind flux.ReleaseKind, getServices serviceQuery, getImages imageSelector, updateJob func(string, ...interface{})) (err error) {

	defer func() { stage.ObserveDuration() }()
	stage = metrics.NewTimer(base.With("stage", "fetch_platform_services"))

	// TODO: Why do we care what is running? The git repo is the source of truth!
	services, err := getServices(inst)
	if err != nil {
		return errors.Wrap(err, "fetching platform services")
	}

	stage.ObserveDuration()
	stage = metrics.NewTimer(base.With("stage", "calculate_regrades"))

	// TODO: What if we added a new container (or image) to a service, which is
	// in the manifests, but not running?
	//
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

	// Apply the new manifests
	res = append(res, r.releaseActionRegradeServices(servicesToRegrade, msg))

	return nil
}

func (r *Releaser) execute(inst *instance.Instance, actions []ReleaseAction, kind flux.ReleaseKind, updateJob func(string, ...interface{})) error {
	rc := NewRepo(inst)
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

func (r *Releaser) releaseActionPrintf(format string, args ...interface{}) ReleaseAction {
	return ReleaseAction{
		Description: fmt.Sprintf(format, args...),
		Do: func(_ *Repo) (res string, err error) {
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
		Do: func(rc *Repo) (res string, err error) {
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
		Do: func(rc *Repo) (res string, err error) {
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

func (r *Releaser) releaseActionUpdatePodController(service flux.ServiceID, regrades []containerRegrade) ReleaseAction {
	var actions []string
	for _, regrade := range regrades {
		actions = append(actions, fmt.Sprintf("%s (%s -> %s)", regrade.container, regrade.current, regrade.target))
	}
	actionList := strings.Join(actions, ", ")

	return ReleaseAction{
		Description: fmt.Sprintf("Update %d images(s) in the resource definition file for %s: %s.", len(regrades), service, actionList),
		Do: func(rc *Repo) (res string, err error) {
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

func (r *Releaser) releaseActionCommitAndPush(msg string) ReleaseAction {
	return ReleaseAction{
		Description: "Commit and push the config repo.",
		Do: func(rc *Repo) (res string, err error) {
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

func (r *Releaser) releaseActionRegradeServices(services []flux.ServiceID, msg string) ReleaseAction {
	return ReleaseAction{
		Description: fmt.Sprintf("Regrade %d service(s): %s.", len(services), strings.Join(service2string(services), ", ")),
		Do: func(rc *Repo) (res string, err error) {
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
			// If we're regrading our own image, we want to do that
			// last, and "asynchronously" (meaning we probably won't
			// see the reply).
			var asyncSpecs []platform.RegradeSpec

			for _, service := range services {
				def, ok := rc.PodControllers[service]
				if !ok {
					results[service] = errors.New("no definition found; skipping regrade")
					continue
				}

				namespace, serviceName := service.Components()
				switch serviceName {
				case FluxServiceName, FluxDaemonName:
					rc.Instance.LogEvent(namespace, serviceName, "Starting regrade (no result expected) "+cause)
					asyncSpecs = append(asyncSpecs, platform.RegradeSpec{
						ServiceID:     service,
						NewDefinition: def,
					})
				default:
					rc.Instance.LogEvent(namespace, serviceName, "Starting regrade "+cause)
					specs = append(specs, platform.RegradeSpec{
						ServiceID:     service,
						NewDefinition: def,
					})
				}
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
				switch serviceName {
				case FluxServiceName, FluxDaemonName:
					continue
				default:
					if err := results[service]; err == nil { // no entry = nil error
						rc.Instance.LogEvent(namespace, serviceName, "Regrade due to "+cause+": done")
					} else {
						rc.Instance.LogEvent(namespace, serviceName, "Regrade due to "+cause+": failed: "+err.Error())
					}
				}
			}

			// Lastly, services for which we don't expect a result
			// (i.e., ourselves). This will kick off the regrade in
			// the daemon, which will cause Kubernetes to restart the
			// service. In the meantime, however, we will have
			// finished recording what happened, as part of a graceful
			// shutdown. So the only thing that goes missing is the
			// result from this regrade call.
			if len(asyncSpecs) > 0 {
				go func() {
					rc.Instance.PlatformRegrade(asyncSpecs)
				}()
			}

			return "", transactionErr
		},
	}
}
