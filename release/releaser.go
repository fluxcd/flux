package release

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	//	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/jobs"
	fluxmetrics "github.com/weaveworks/flux/metrics"
	"github.com/weaveworks/flux/notifications"
	"github.com/weaveworks/flux/platform"
	"github.com/weaveworks/flux/platform/kubernetes"
)

const FluxServiceName = "fluxsvc"
const FluxDaemonName = "fluxd"

type Releaser struct {
	instancer instance.Instancer
	metrics   Metrics
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

type ServiceUpdate struct {
	ServiceID     flux.ServiceID
	Service       platform.Service
	ManifestPath  string
	ManifestBytes []byte
	Updates       []flux.ContainerUpdate
}

// These represent the side-effects that calculating and applying the
// release can have: namely, outputting status messages, and updating
// a result report.
type statusFn func(string, ...interface{})
type resultFn func(resultSoFar flux.ReleaseResult)

func (r *Releaser) Handle(job *jobs.Job, updater jobs.JobUpdater) ([]jobs.Job, error) {
	logStatus := func(format string, args ...interface{}) {
		status := fmt.Sprintf(format, args...)
		job.Status = status
		job.Log = append(job.Log, status)
		updater.UpdateJob(*job)
	}
	updateResult := func(result flux.ReleaseResult) {
		// TODO update in the job structure -- once it has that field.
	}

	// The job gets handed down through methods just so it can be used
	// to construct a Release for the (possible) notification, which
	// is a bit awkward; but we can factor it out once we have a less
	// coupled way of dealing with release notifications (e.g., as a
	// job itself).
	return r.release(job.Instance, job, logStatus, updateResult)
}

func (r *Releaser) release(instanceID flux.InstanceID, job *jobs.Job, logStatus statusFn, report resultFn) (_ []jobs.Job, err error) {
	spec := job.Params.(jobs.ReleaseJobParams)
	defer func(started time.Time) {
		r.metrics.ReleaseDuration.With(
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
			fluxmetrics.LabelReleaseType, flux.ReleaseSpec(spec).ReleaseType(),
			fluxmetrics.LabelReleaseKind, string(spec.Kind),
		).Observe(time.Since(started).Seconds())
	}(time.Now())

	logStatus("Calculating updates for release.")
	inst, err := r.instancer.Get(instanceID)
	if err != nil {
		return nil, err
	}

	// We time each stage of this process, and expose as metrics.
	var timer *metrics.Timer

	// Preparation: we always need the repository
	rc := NewReleaseContext(inst)
	defer rc.Clean()
	logStatus("Cloning git repository.")
	timer = r.metrics.NewStageTimer("clone_repository")
	if err = rc.CloneRepo(); err != nil {
		return nil, err
	}
	timer.ObserveDuration()

	// From here in, we collect the results of the calculations.
	results := flux.ReleaseResult{}

	// Figure out the services involved.
	logStatus("Finding defined services.")
	timer = r.metrics.NewStageTimer("select_services")
	var updates []*ServiceUpdate
	updates, err = r.selectServices(rc, &spec, results, logStatus)
	timer.ObserveDuration()
	if err != nil {
		return nil, err
	}
	logStatus("Found %d services.", len(updates))
	report(results)

	logStatus("Looking up images.")
	if spec.ImageSpec != flux.ImageSpecNone {
		timer = r.metrics.NewStageTimer("lookup_images")
		// Figure out how the services are to be updated.
		updates, err = r.calculateImageUpdates(rc, updates, &spec, results, logStatus)
		timer.ObserveDuration()
		if err != nil {
			return nil, err
		}
		report(results)
	}

	// At this point we have have filtered the updates we can do down
	// to nothing. Check and exit early if so
	if len(updates) == 0 {
		logStatus("No updates to do, finishing.")
		report(results)
		return nil, nil
	}

	if spec.Kind == flux.ReleaseKindExecute {
		if spec.ImageSpec != flux.ImageSpecNone {
			logStatus("Pushing changes.")
			timer = r.metrics.NewStageTimer("push_changes")
			err = r.pushChanges(rc, updates, &spec)
			timer.ObserveDuration()
			if err != nil {
				return nil, err
			}
		}

		logStatus("Applying changes.")
		timer = r.metrics.NewStageTimer("apply_changes")
		err = r.applyChanges(rc, updates, &spec, results)
		timer.ObserveDuration()
		// Report on success or failure of the application above
		timer = r.metrics.NewStageTimer("send_notifications")
		err = sendNotifications(rc, err, results, job)
		timer.ObserveDuration()
	}
	report(results)

	return nil, err
}

func sendNotifications(rc *ReleaseContext, executeErr error, results flux.ReleaseResult, job *jobs.Job) error {
	cfg, err := rc.Instance.GetConfig()
	if err != nil {
		if executeErr == nil {
			return errors.Wrap(err, "sending notifications")
		}
		return executeErr
	}

	status := flux.ReleaseStatusSuccess
	if executeErr != nil {
		status = flux.ReleaseStatusFailed
	}
	// Filling this from the job is a temporary migration hack. Ideally all
	// the release info should be stored on the release object in a releases
	// table, and the job should really just have a pointer to that.
	err = notifications.Release(cfg, flux.Release{
		ID:        flux.ReleaseID(job.ID),
		CreatedAt: job.Submitted,
		StartedAt: job.Claimed,
		// TODO: fetch the job and look this up so it matches
		// (which must be done after completing the job)
		EndedAt:  time.Now().UTC(),
		Done:     true,
		Priority: job.Priority,
		Status:   status,
		Log:      job.Log,

		Spec:   flux.ReleaseSpec(job.Params.(jobs.ReleaseJobParams)),
		Result: results,
	}, executeErr)
	if err != nil {
		if executeErr == nil {
			return errors.Wrap(err, "sending notifications")
		}
	}
	return nil
}

// Take the spec given in the job, and figure out which services are
// in question based on the running services and those defined in the
// repo. Fill in the release results along the way.
func (r *Releaser) selectServices(rc *ReleaseContext, releaseJob *jobs.ReleaseJobParams, results flux.ReleaseResult, logStatus statusFn) ([]*ServiceUpdate, error) {
	conf, err := rc.Instance.GetConfig()
	if err != nil {
		return nil, err
	}
	lockedSet := LockedServices(conf)

	excludedSet := flux.ServiceIDSet{}
	excludedSet.Add(releaseJob.Excludes)

	// For backwards-compatibility, there's two fields: ServiceSpec
	// and ServiceSpecs. An entry in ServiceSpec takes precedence.
	switch releaseJob.ServiceSpec {
	case flux.ServiceSpec(""):
		ids := []flux.ServiceID{}
		for _, s := range releaseJob.ServiceSpecs {
			if s == flux.ServiceSpecAll {
				return rc.SelectServices(nil, lockedSet, excludedSet, results, logStatus)
			}
			id, err := flux.ParseServiceID(string(s))
			if err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
		return rc.SelectExactServices(ids, lockedSet, excludedSet, results, logStatus)
	case flux.ServiceSpecAll:
		return rc.SelectServices(nil, lockedSet, excludedSet, results, logStatus)
	default:
		id, err := flux.ParseServiceID(string(releaseJob.ServiceSpec))
		if err != nil {
			return nil, err
		}
		return rc.SelectExactServices([]flux.ServiceID{id}, lockedSet, excludedSet, results, logStatus)
	}
}

// Find all the image updates that should be performed, and do
// replacements. (For a dry-run, we don't strictly need to do the
// replacements, since we won't be committing any changes back;
// however we do want to see if we *can* do the replacements, because
// if not, it indicates there's likely some problem with the running
// system vs the definitions given in the repo.)
func (r *Releaser) calculateImageUpdates(rc *ReleaseContext, updates []*ServiceUpdate, releaseJob *jobs.ReleaseJobParams, results flux.ReleaseResult, logStatus statusFn) ([]*ServiceUpdate, error) {
	var images instance.ImageMap
	var err error

	switch releaseJob.ImageSpec {
	case flux.ImageSpecNone:
		images = instance.ImageMap{}
	case flux.ImageSpecLatest:
		images, err = rc.CollectAvailableImages(updates)
	default:
		var image flux.ImageID
		image, err = releaseJob.ImageSpec.AsID()
		if err == nil {
			images, err = rc.Instance.ExactImages([]flux.ImageID{image})
		}
	}

	if err != nil {
		return nil, err
	}

	// Do all the updates that could be written out
	return rc.CalculateContainerUpdates(updates, images, results, logStatus)
}

func (r *Releaser) pushChanges(rc *ReleaseContext, updates []*ServiceUpdate, spec *jobs.ReleaseJobParams) error {
	err := rc.WriteUpdates(updates)
	if err != nil {
		return err
	}

	commitMsg := CommitMessageFromReleaseSpec(spec)
	// TODO account for "nothing changed" message, which may be returned here
	_, err = rc.CommitAndPush(commitMsg)
	return err
}

// applyChanges effects the calculated changes on the platform.
func (r *Releaser) applyChanges(rc *ReleaseContext, updates []*ServiceUpdate, releaseJob *jobs.ReleaseJobParams, results flux.ReleaseResult) error {
	// Collect definitions for each service release.
	var defs []platform.ServiceDefinition
	// If we're regrading our own image, we want to do that
	// last, and "asynchronously" (meaning we probably won't
	// see the reply).
	var asyncDefs []platform.ServiceDefinition

	for _, update := range updates {
		namespace, serviceName := update.ServiceID.Components()
		switch serviceName {
		case FluxServiceName, FluxDaemonName:
			rc.Instance.LogEvent(namespace, serviceName, "Starting. (no result expected)")
			asyncDefs = append(asyncDefs, platform.ServiceDefinition{
				ServiceID:     update.ServiceID,
				NewDefinition: update.ManifestBytes,
				Async:         true,
			})
		default:
			rc.Instance.LogEvent(namespace, serviceName, "Starting")
			defs = append(defs, platform.ServiceDefinition{
				ServiceID:     update.ServiceID,
				NewDefinition: update.ManifestBytes,
			})
		}
	}

	transactionErr := rc.Instance.PlatformApply(defs)
	if transactionErr != nil {
		switch err := transactionErr.(type) {
		case platform.ApplyError:
			for id, applyErr := range err {
				results[id] = flux.ServiceResult{
					Status: flux.ReleaseStatusFailed,
					Error:  applyErr.Error(),
				}
			}
		default: // assume everything that was planned failed, if
			// there was a coverall error
			for id, _ := range results {
				if results[id].Status == flux.ReleaseStatusPending {
					results[id] = flux.ServiceResult{
						Status: flux.ReleaseStatusUnknown,
						Error:  transactionErr.Error(),
					}
				}
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
		rc.Instance.PlatformApply(asyncDefs)
	}

	return transactionErr
}

func CommitMessageFromReleaseSpec(spec *jobs.ReleaseJobParams) string {
	image := strings.Trim(spec.ImageSpec.String(), "<>")
	var services []string
	for _, s := range spec.ServiceSpecs {
		services = append(services, strings.Trim(s.String(), "<>"))
	}
	return fmt.Sprintf("Release %s to %s", image, strings.Join(services, ", "))
}

// Additions to ReleaseContext for above

// CollectAvailableImages goes through a set of updateable services,
// and constructs a map with all the images available in it.
func (rc *ReleaseContext) CollectAvailableImages(updateable []*ServiceUpdate) (instance.ImageMap, error) {
	var servicesToCheck []platform.Service
	for _, update := range updateable {
		servicesToCheck = append(servicesToCheck, update.Service)
	}
	// TODO factor this out to rc.LookForNewImages(updates), it's used
	// in releaser too
	return rc.Instance.CollectAvailableImages(servicesToCheck)
}

// CalculateContainerUpdates looks through the updateable services and
// the images that have been found, and figures out the exact
// containers in each service that can be updated. For services with
// containers to be updated, it rewrites the manifest file. It also
// keeps track of what happened, in the Result. Returns only the
// updates that could be made.
func (rc *ReleaseContext) CalculateContainerUpdates(updateable []*ServiceUpdate, images instance.ImageMap, result flux.ReleaseResult, logStatus statusFn) ([]*ServiceUpdate, error) {
	var updates []*ServiceUpdate

	for _, update := range updateable {
		containers, err := update.Service.ContainersOrError()
		if err != nil {
			logStatus("Failing service %s: %s", update.ServiceID, err.Error())
			result[update.ServiceID] = flux.ServiceResult{
				Status: flux.ReleaseStatusFailed,
				Error:  err.Error(),
			}
			continue
		}

		var containerUpdates []flux.ContainerUpdate
		for _, container := range containers {
			currentImageID, err := flux.ParseImageID(container.Image)
			if err != nil {
				// We may hope not to find a malformed image ID, but
				// anything is possible.
				return nil, err
			}

			latestImage := images.LatestImage(currentImageID.Repository())
			if latestImage == nil {
				continue
			}

			if currentImageID == latestImage.ID {
				continue
			}

			update.ManifestBytes, err = kubernetes.UpdatePodController(update.ManifestBytes, latestImage.ID, ioutil.Discard)
			if err != nil {
				// TODO do I want to fail utterly, or just this service?
				return nil, err
			}

			logStatus("Updating service %s container %s: %s -> :%s", update.ServiceID, container.Name, currentImageID, latestImage.ID.Tag)
			containerUpdates = append(containerUpdates, flux.ContainerUpdate{
				Container: container.Name,
				Current:   currentImageID,
				Target:    latestImage.ID,
			})

		}
		if len(containerUpdates) > 0 {
			update.Updates = containerUpdates
			updates = append(updates, update)
			result[update.ServiceID] = flux.ServiceResult{
				Status:       flux.ReleaseStatusPending,
				PerContainer: containerUpdates,
			}
		} else {
			logStatus("Skipping service %s, no container images to update", update.ServiceID)
		}
	}

	return updates, nil
}

func (rc *ReleaseContext) WriteUpdates(updates []*ServiceUpdate) error {
	for _, update := range updates {
		fi, err := os.Stat(update.ManifestPath)
		if err != nil {
			return err
		}
		if err = ioutil.WriteFile(update.ManifestPath, update.ManifestBytes, fi.Mode()); err != nil {
			return err
		}
	}
	return nil
}

// /Additions
