package release

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
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
	Description string                                `json:"description"`
	Do          func(*ReleaseContext) (string, error) `json:"-"`
	Result      string                                `json:"result"`
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
			params.ImageSpec,
			params.Kind,
			services,
			images,
			updateJob,
		),
		params.Kind,
		updateJob,
	)
}

func (r *Releaser) plan(
	msg string,
	inst *instance.Instance,
	imageSpec flux.ImageSpec,
	kind flux.ReleaseKind,
	getServices serviceQuery,
	getImages imageSelector,
	updateJob func(string, ...interface{}),
) []ReleaseAction {
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

	// TODO: Implement this
	var notifications []Notification

	res = append(res, r.releaseActionPrintf(msg)) // TODO: Replace this with a better title-printer.
	res = append(res, r.planActions(inst, imageSpec, getImages, getServices)...)
	res = append(res, r.executeActions(msg, imageSpec, kind, notifications)...)
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

func (r *Releaser) planActions(inst *instance.Instance, imageSpec flux.ImageSpec, getImages imageSelector, getServices serviceQuery) []ReleaseAction {
	return []ReleaseAction{
		r.releaseActionClone(),
		r.releaseActionFindDefinitions(getServices),
		r.releaseActionCheckForNewImages(imageSpec, getImages),
		r.releaseActionUpdateDefinitions(imageSpec, getImages, getServices),
	}
}

func (r *Releaser) executeActions(commitMsg string, imageSpec flux.ImageSpec, kind flux.ReleaseKind, notifications []Notification) []ReleaseAction {
	return []ReleaseAction{
		r.releaseActionCommitAndPush(imageSpec, kind, commitMsg),
		r.releaseActionApplyToPlatform(kind, commitMsg),
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
			return "Cloned " + rc.RepoURL(), nil
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
			// TODO: take the platform here and *ask* it what type it is, instead of
			// assuming kubernetes.
			allServiceDefinitions, err := kubernetes.DefinedServices(resourcePath)
			if err != nil {
				return "", errors.Wrapf(err, "finding resource definition files for %s", getServiceDefinitions)
			}
			definedServiceIDs := make([]flux.ServiceID, len(allServiceDefinitions))
			for id := range allServiceDefinitions {
				definedServiceIDs = append(definedServiceIDs, id)
			}
			if len(definedServiceIDs) <= 0 {
				return "", errors.New("no resource definition files found")
			}

			serviceIDs, err := getServiceDefinitions.SelectServices(definedServiceIDs)
			if err != nil {
				return "", errors.Wrapf(err, "selecting definition files for %s", getServiceDefinitions)
			}

			if len(serviceIDs) <= 0 {
				return "", errors.New("no resource definition files selected")
			}

			for _, id := range serviceIDs {
				if paths := allServiceDefinitions[id]; len(paths) > 1 {
					sort.Strings(paths)
					return "", fmt.Errorf("multiple resource definition files found for %s: %s", id, strings.Join(paths, ", "))
				}
			}

			// Load the actual definitions for services we've selected.
			for _, id := range serviceIDs {
				rc.ServiceDefinitions[id] = map[string][]byte{}
				for _, path := range allServiceDefinitions[id] {
					definition, err := ioutil.ReadFile(path)
					if err != nil {
						return "", errors.Wrapf(err, "reading definition file for %s: %s", id, path)
					}
					rc.ServiceDefinitions[id][path] = definition
				}
			}

			// Parse service definitions to find currently used images for each service
			// TODO: take the platform here and *ask* it what type it is, instead of
			// assuming kubernetes.
			filesCount := 0
			for service, files := range rc.ServiceDefinitions {
				filesCount += len(files)
				images := map[flux.ImageID]struct{}{}
				for path, definition := range files {
					found, err := kubernetes.ImagesForDefinition(definition)
					if err != nil {
						return "", errors.Wrapf(err, "parsing definition file: %s", path)
					}
					for _, image := range found {
						images[image] = struct{}{}
					}
				}
				for image := range images {
					rc.ServiceImages[service] = append(rc.ServiceImages[service], image)
				}
				flux.ImageIDSlice(rc.ServiceImages[service]).Sort()
			}

			return fmt.Sprintf("Found %d definition files", filesCount), nil
		},
	}
}

func (r *Releaser) releaseActionCheckForNewImages(imageSpec flux.ImageSpec, getImages imageSelector) ReleaseAction {
	return ReleaseAction{
		Name:        "check_for_new_images",
		Description: fmt.Sprintf("Check registry for %s", getImages),
		Do: func(rc *ReleaseContext) (res string, err error) {
			// Handle --no-update-image releases here! No need to look up new images.
			// Calling getImages would be a noop, but this way we output a nicer
			// message.
			if imageSpec == flux.ImageSpecNone {
				return "Skipped", nil
			}

			// Fetch the image metadata
			rc.Images, err = getImages.SelectImages(rc.Instance, rc.ServiceImages)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("Found %d new images", len(rc.Images)), nil
		},
	}
}

func (r *Releaser) releaseActionUpdateDefinitions(imageSpec flux.ImageSpec, getImages imageSelector, getServices serviceQuery) ReleaseAction {
	return ReleaseAction{
		Name:        "update_definitions",
		Description: fmt.Sprintf("Update definition files for %s to %s", getServices, getImages),
		Do: func(rc *ReleaseContext) (res string, err error) {
			// Handle --no-update-image releases here! Need to apply existing definitions instead.
			if imageSpec == flux.ImageSpecNone {
				rc.UpdatedDefinitions = rc.ServiceDefinitions
				return "Skipped", nil
			}

			definitionCount := 0
			for service, images := range rc.ServiceImages {
				// Update all definition files for this service. (should only be one)
				for path, definition := range rc.ServiceDefinitions[service] {
					// We keep overwriting the same def, to handle multiple
					// images in a single file.
					updatedDefinition := definition
					definitionChanged := false
					for _, image := range images {
						target := rc.Images.LatestImage(image.Repository())
						if target == nil {
							continue
						}

						if image == target.ID {
							// Definition is already up to date. Nothing to do.
							// TODO: Add a log or output of this.
							continue
						}

						// UpdateDefinition parses the target (new) image
						// name, extracts the repository, and only mutates the line(s)
						// in the definition that match it. So for the time being we
						// ignore the current image. UpdateDefinition could be
						// updated, if necessary.
						updatedDefinition, err = kubernetes.UpdateDefinition(updatedDefinition, target.ID, ioutil.Discard)
						if err != nil {
							return "", errors.Wrapf(err, "updating definition for %s", target)
						}
						definitionChanged = true
					}
					if definitionChanged {
						if _, ok := rc.UpdatedDefinitions[service]; !ok {
							rc.UpdatedDefinitions[service] = map[string][]byte{}
						}
						rc.UpdatedDefinitions[service][path] = updatedDefinition
						definitionCount++
					}
				}
			}
			return fmt.Sprintf("Updated %d definition files for %d services", definitionCount, len(rc.UpdatedDefinitions)), nil
		},
	}
}

func (r *Releaser) releaseActionCommitAndPush(imageSpec flux.ImageSpec, kind flux.ReleaseKind, commitMsg string) ReleaseAction {
	return ReleaseAction{
		Name:        "commit_and_push",
		Description: "Commit and push the definitions repo",
		Do: func(rc *ReleaseContext) (res string, err error) {
			if imageSpec == flux.ImageSpecNone || kind != flux.ReleaseKindExecute {
				return "Skipped", nil
			}

			if len(rc.UpdatedDefinitions) == 0 {
				return "No definitions updated, nothing to commit", nil
			}

			// Write each changed definition file back, so commit/push works.
			for service, definitions := range rc.UpdatedDefinitions {
				for path, definition := range definitions {
					fi, err := os.Stat(path)
					if err != nil {
						return "", errors.Wrapf(err, "writing new definition file for %s: %s", service, path)
					}

					if err := ioutil.WriteFile(path, definition, fi.Mode()); err != nil {
						return "", errors.Wrapf(err, "writing new definition file for %s: %s", service, path)
					}
				}
			}

			if fi, err := os.Stat(rc.WorkingDir); err != nil || !fi.IsDir() {
				return "", fmt.Errorf("the repo path (%s) is not valid", rc.WorkingDir)
			}
			result, err := rc.CommitAndPush(commitMsg)
			if err == nil && result == "" {
				return "Pushed commit: " + commitMsg, nil
			}
			return result, err
		},
	}
}

func (r *Releaser) releaseActionApplyToPlatform(kind flux.ReleaseKind, commitMsg string) ReleaseAction {
	return ReleaseAction{
		Name: "apply_to_platform",
		// TODO: take the platform here and *ask* it what type it is, instead of
		// assuming kubernetes.
		Description: "Rolling-update to Kubernetes",
		Do: func(rc *ReleaseContext) (res string, err error) {
			if kind != flux.ReleaseKindExecute {
				return "Skipped", nil
			}

			// We'll collect results for each service release.
			results := map[flux.ServiceID]error{}

			// Collect definitions for each service release.
			var defs []platform.ServiceDefinition
			// If we're releasing our own image, we want to do that
			// last, and "asynchronously" (meaning we probably won't
			// see the reply).
			var asyncDefs []platform.ServiceDefinition

			// Apply each changed definition to the platform, so commit/push works.
			cause := strconv.Quote(commitMsg)
			for service, definitions := range rc.UpdatedDefinitions {
				if len(definitions) == 0 {
					results[service] = errors.New("no definitions found; skipping apply")
					continue
				}

				for _, definition := range definitions {
					namespace, serviceName := service.Components()
					newDefinition := platform.ServiceDefinition{
						ServiceID:     service,
						NewDefinition: definition,
					}
					switch serviceName {
					case FluxServiceName, FluxDaemonName:
						rc.Instance.LogEvent(namespace, serviceName, "Starting "+cause+". (no result expected)")
						asyncDefs = append(asyncDefs, newDefinition)
					default:
						rc.Instance.LogEvent(namespace, serviceName, "Starting "+cause)
						defs = append(defs, newDefinition)
					}
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
					for service := range rc.UpdatedDefinitions {
						results[service] = transactionErr
					}
				}
			}

			// Report individual service release results.
			// TODO: Integrate Regrade -> Apply changes here
			// TODO: Record the changes into the ReleaseContext, so we can send
			// notifications of them in the next step.
			for service := range rc.UpdatedDefinitions {
				namespace, serviceName := service.Components()
				switch serviceName {
				case FluxServiceName, FluxDaemonName:
					continue
				default:
					if err := results[service]; err == nil { // no entry = nil error
						rc.Instance.LogEvent(namespace, serviceName, commitMsg+". done")
					} else {
						rc.Instance.LogEvent(namespace, serviceName, commitMsg+". failed: "+err.Error())
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

func (r *Releaser) releaseActionSendNotifications(kind flux.ReleaseKind, notifications []Notification) ReleaseAction {
	return ReleaseAction{
		Name:        "send_notifications",
		Description: fmt.Sprintf("Send notifications to %s", notifications),
		Do: func(rc *ReleaseContext) (res string, err error) {
			if kind != flux.ReleaseKindExecute {
				return "Skipped", nil
			}
			// TODO: We should run this even if some other steps have failed... So we
			// can report failed releases.
			return "", fmt.Errorf("TODO")
		},
	}
}

func (r *Releaser) execute(metric metrics.Histogram, inst *instance.Instance, actions []ReleaseAction, kind flux.ReleaseKind, updateJob func(string, ...interface{})) error {
	rc := NewReleaseContext(inst)
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
			return nil
		}()
		if err != nil {
			return err
		}
	}

	return nil
}
