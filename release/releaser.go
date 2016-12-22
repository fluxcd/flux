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
	Name        string
	Description string                      `json:"description"`
	Do          func(*Repo) (string, error) `json:"-"`
	Result      string                      `json:"result"`
}

func (r *Releaser) Handle(job *jobs.Job, updater jobs.JobUpdater) (followUps []jobs.Job, err error) {
	params := job.Params.(jobs.ReleaseJobParams)
	releaseType := params.ReleaseType()
	defer func(begin time.Time) {
		r.metrics.ReleaseDuration.With(
			"method", releaseType,
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

	return nil, r.execute(
		r.metrics.StageDuration.With("method", releaseType),
		inst,
		r.plan(
			fmt.Sprintf("Release %s to %s", images, services),
			inst,
			params.Kind,
			services,
			images,
			updateJob,
		),
		kind,
		updateJob,
	)
}

func (r *Releaser) plan(msg string, inst *instance.Instance, kind flux.ReleaseKind, getServices serviceQuery, getImages imageSelector, updateJob func(string, ...interface{})) []ReleaseAction {
	var res []ReleaseAction
	// TODO: consider maybe having Stages, then actions
	// Like: [{Name: "gather_materials", Actions: []}, ]
	// So that you can do output like:
	//
	//   Release latest images to all services:
	//
	//     Plan
	//       1) Clone definition repository
	//          ... OK. Cloned git@github.com:weaveworks/service-conf
	//       2) Find definition files for all services
	//          ... OK. Found 14 definition files.
	//       3) Check registry for latest images
	//          ... OK. Found 2 new images.
	//       4) Update definition files to latest images
	//          ... Fail: no definition files found for foobar
	//
	//     Execute [Skipped: Dry-Run]
	//       5) Commit and push new definitions
	//
	//       6) Rolling-update to Kubernetes
	//
	//       7) Send notifications to Slack (#cloud)
	//
	//   Result: Fail: no definition files found for foobar
	//
	res = append(res, r.releaseActionPrintf(msg)) // TODO: Replace this with a better title-printer.
	res = append(res, r.planActions(inst, getImages)...)
	res = append(res, r.executeActions(msg, getServices, kind)...)
	return res
}

func (r *Releaser) releaseActionPrintf(format string, args ...interface{}) ReleaseAction {
	return ReleaseAction{
		Name:        "printf",
		Description: fmt.Sprintf(format, args...),
		Do: func(_ *ReleaseContext) (res string, err error) {
			return "", nil
		},
	}
}

func (r *Releaser) planActions(inst *instance.Instance, getServices serviceQuery, getImages imageSelector) []ReleaseAction {
	return []ReleaseAction{
		r.releaseActionClone(inst),
		r.releaseActionFindDefinitions(getServices),
		r.releaseActionCheckForNewImages(getImages),
		r.releaseActionUpdateDefinitions(getServices),
	}
}

func (r *Releaser) executeActions(commitMsg string, kind flux.ReleaseKind, notifications []Notifications) []ReleaseAction {
	return []ReleaseAction{
		r.releaseActionCommitAndPush(kind, commitMsg),
		r.releaseActionApplyToPlatform(kind),
		r.releaseActionSendNotifications(kind, notifications),
	}
}

func (r *Releaser) releaseActionClone() ReleaseAction {
	return ReleaseAction{
		Name:        "clone",
		Description: "Clone the definition repository",
		Do: func(rc *ReleaseContext) (res string, err error) {
			err = rc.CloneRepo()
			if err != nil {
				return "", errors.Wrap(err, "clone the definition repository")
			}
			return fmt.Sprintf("Cloned %s", rs.URL), nil
		},
	}
}

func (r *Releaser) releaseActionFindDefinitions(getServiceDefinitions serviceQuery) ReleaseAction {
	return ReleaseAction{
		Name:        "find_definitions",
		Description: fmt.Sprintf("Find definition files for %s", getServiceDefinitions),
		Do: func(rc *ReleaseContext) (res string, err error) {
			resourcePath := rc.RepoPath()
			if fi, err := os.Stat(resourcePath); err != nil || !fi.IsDir() {
				return "", fmt.Errorf("the resource path (%s) is not valid", resourcePath)
			}

			// TODO: The files returned here should actually have a "position" of the
			// definition, for multi-document and list-style k8s manifests
			services, err := getServiceDefinitions(resourcePath)
			if err != nil {
				return "", errors.Wrapf(err, "finding resource definition files for %s", getServiceDefinitions)
			}
			if len(services) <= 0 {
				return nil, errors.New("no resource definition files found")
			}
			for service, files := range services {
				if len(files) > 1 {
					return "", fmt.Errorf("multiple resource definition files found for %s: %s", service, strings.Join(files, ", "))
				}
			}

			rc.ServiceDefinitions = services
			return fmt.Sprintf("Found %d definition files", len(services)), nil
		},
	}
}

func (r *Releaser) releaseActionCheckForNewImages(getImages imageSelector) ReleaseAction {
	return ReleaseAction{
		Name:        "check_for_new_images",
		Description: fmt.Sprintf("Check registry for %s", getImages),
		Do: func(rc *ReleaseContext) (res string, err error) {
			rc.Images, err = getImages(rc.Instance, rc.Services)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("Found %d new images", len(rc.Images)), nil

			/*
				resourcePath := rc.RepoPath()
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
			*/
		},
	}
}

func (r *Releaser) releaseActionUpdateDefinitions(getServices serviceQuery, getImages imageSelector) ReleaseAction {
	return ReleaseAction{
		Name:        "update_definitions",
		Description: fmt.Sprintf("Update definition files for %s to %s", getServices, getImages),
		Do: func(rc *ReleaseContext) (res string, err error) {
			return "", fmt.Errorf("TODO")

			/*
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

				for service, files := range rc.ServiceDefinitions {
					def, err := ioutil.ReadFile(files[0]) // TODO(mb) not multi-doc safe
					if err != nil {
						return "", err
					}
					fi, err := os.Stat(files[0])
					if err != nil {
						return "", err
					}
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
			*/
		},
	}
}

func (r *Releaser) releaseActionCommitAndPush(kind flux.ReleaseKind, msg string) ReleaseAction {
	return ReleaseAction{
		Name:        "commit_and_push",
		Description: "Commit and push the definitions repo",
		Do: func(rc *ReleaseContext) (res string, err error) {
			if kind != flux.ReleaseKindExecute {
				return "Skipped", nil
			}
			return "", fmt.Errorf("TODO")
			/*
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
			*/
		},
	}
}

func (r *Releaser) releaseActionApplyToPlatform(kind flux.ReleaseKind) ReleaseAction {
	return ReleaseAction{
		Name: "apply_to_platform",
		// TODO: take the platform here and *ask* it what type it is, instead of
		// assuming kubernetes.
		Description: "Rolling-update to Kubernetes",
		Do: func(rc *ReleaseContext) (res string, err error) {
			if kind != flux.ReleaseKindExecute {
				return "Skipped", nil
			}
			return "", fmt.Errorf("TODO")
		},
	}
}

func (r *Releaser) releaseActionSendNotifications(kind flux.ReleaseKind, notifications []Notifications) ReleaseAction {
	return ReleaseAction{
		Name:        "send_notifications",
		Description: fmt.Sprintf("Send notifications to %s", notifications),
		Do: func(rc *ReleaseContext) (res string, err error) {
			if kind != flux.ReleaseKindExecute {
				return "Skipped", nil
			}
			return "", fmt.Errorf("TODO")
		},
	}
}

func (r *Releaser) execute(metric metrics.Histogram, inst *instance.Instance, actions []ReleaseAction, kind flux.ReleaseKind, updateJob func(string, ...interface{})) error {
	rc := NewRepo(inst)
	defer rc.Clean()

	for i, action := range actions {
		err := func() (err error) {
			defer func(begin time.Time) {
				metric.With(
					"action", action.Name,
					"success", fmt.Sprint(err == nil),
				).Observe(time.Since(begin).Seconds())
			}(time.Now())

			updateJob(action.Description)
			inst.Log("description", action.Description)

			if action.Do == nil || kind != flux.ReleaseKindExecute {
				return nil
			}

			result, err := action.Do(rc)
			if err != nil {
				updateJob(err.Error())
				inst.Log("err", err)
				actions[i].Result = "Failed: " + err.Error()
				return err
			}
			result = strings.Join([]string{"OK", result}, ". ")
			if result != "OK" {
				updateJob(result)
			}
			actions[i].Result = result
		}()
		if err != nil {
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
