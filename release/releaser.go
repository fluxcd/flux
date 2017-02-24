package release

import (
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
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
}

func NewReleaser(
	instancer instance.Instancer,
) *Releaser {
	return &Releaser{
		instancer: instancer,
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
		job.Result = result
		updater.UpdateJob(*job)
	}

	// The job gets handed down through methods just so it can be used
	// to construct a Release for the (possible) notification, which
	// is a bit awkward; but we can factor it out once we have a less
	// coupled way of dealing with release notifications (e.g., as a
	// job itself).
	return r.release(job.Instance, job, logStatus, updateResult)
}

func (r *Releaser) release(instanceID flux.InstanceID, job *jobs.Job, logStatus statusFn, report resultFn) (_ []jobs.Job, err error) {
	spec := job.Params.(jobs.ReleaseJobParams).Spec()
	defer func(started time.Time) {
		releaseDuration.With(
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

	inst.Logger = log.NewContext(inst.Logger).With("release-id", string(job.ID))

	// We time each stage of this process, and expose as metrics.
	var timer *metrics.Timer

	// Preparation: we always need the repository
	rc := NewReleaseContext(inst)
	defer rc.Clean()
	logStatus("Cloning git repository.")
	timer = NewStageTimer("clone_repository")
	if err = rc.CloneRepo(); err != nil {
		return nil, err
	}
	timer.ObserveDuration()

	// From here in, we collect the results of the calculations.
	results := flux.ReleaseResult{}

	// Figure out the services involved.
	logStatus("Finding defined services.")
	timer = NewStageTimer("select_services")
	var updates []*ServiceUpdate
	updates, err = selectServices(rc, &spec, results, logStatus)
	timer.ObserveDuration()
	if err != nil {
		return nil, err
	}
	logStatus("Found %d services.", len(updates))
	report(results)

	// Look up images, and calculate updates, if we've been asked to
	if spec.ImageSpec != flux.ImageSpecNone {
		logStatus("Looking up images.")
		timer = NewStageTimer("lookup_images")
		// Figure out how the services are to be updated.
		updates, err = calculateImageUpdates(rc.Instance, updates, &spec, results, logStatus)
		timer.ObserveDuration()
		if err != nil {
			return nil, err
		}
		report(results)
	}

	// At this point we may have filtered the updates we can do down
	// to nothing. Check and exit early if so.
	if len(updates) == 0 {
		logStatus("No updates to do, finishing.")
		return nil, nil
	}

	// If it's a dry run, we're done.
	if spec.Kind == flux.ReleaseKindPlan {
		return nil, nil
	}

	if spec.ImageSpec != flux.ImageSpecNone {
		logStatus("Pushing changes.")
		timer = NewStageTimer("push_changes")
		err = rc.PushChanges(updates, &spec)
		timer.ObserveDuration()
		if err != nil {
			return nil, err
		}
	}

	logStatus("Applying changes.")
	timer = NewStageTimer("apply_changes")
	applyErr := applyChanges(rc.Instance, updates, results)
	timer.ObserveDuration()

	status := flux.ReleaseStatusSuccess
	if applyErr != nil {
		status = flux.ReleaseStatusFailed
	}
	release := flux.Release{
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

		Spec:   job.Params.(jobs.ReleaseJobParams).Spec(),
		Result: results,
	}

	// Report on success or failure of the application above.
	timer = NewStageTimer("send_notifications")
	notifyErr := sendNotifications(rc.Instance, applyErr, release)
	timer.ObserveDuration()

	// Log the event into the history
	timer = NewStageTimer("log_event")
	err = logEvent(rc.Instance, notifyErr, release)
	timer.ObserveDuration()

	report(results)

	return nil, err
}

// `logEvent` expects the result of applying updates, and records an event in
// the history about the release taking place. It returns the origin error if
// that was non-nil, otherwise the result of the attempted logging.
func logEvent(inst *instance.Instance, executeErr error, release flux.Release) error {
	errorMessage := ""
	logLevel := flux.LogLevelInfo
	if executeErr != nil {
		errorMessage = executeErr.Error()
		logLevel = flux.LogLevelError
	}

	var serviceIDs []flux.ServiceID
	for _, id := range release.Result.ServiceIDs() {
		serviceIDs = append(serviceIDs, flux.ServiceID(id))
	}

	err := inst.LogEvent(flux.Event{
		ServiceIDs: serviceIDs,
		Type:       flux.EventRelease,
		StartedAt:  release.StartedAt,
		EndedAt:    release.EndedAt,
		LogLevel:   logLevel,
		Metadata: flux.ReleaseEventMetadata{
			Release: release,
			Error:   errorMessage,
		},
	})
	if err != nil {
		if executeErr == nil {
			return errors.Wrap(err, "logging event")
		}
	}
	return executeErr
}

// `sendNotifications` expects the result of applying updates, and
// sends notifications indicating success or failure. It returns the
// origin error if that was non-nil, otherwise the result of the
// attempted notifications.
func sendNotifications(inst *instance.Instance, executeErr error, release flux.Release) error {
	cfg, err := inst.GetConfig()
	if err != nil {
		if executeErr == nil {
			return errors.Wrap(err, "sending notifications")
		}
		return executeErr
	}

	// Filling this from the job is a temporary migration hack. Ideally all
	// the release info should be stored on the release object in a releases
	// table, and the job should really just have a pointer to that.
	err = notifications.Release(cfg, release, executeErr)
	if err != nil {
		if executeErr == nil {
			return errors.Wrap(err, "sending notifications")
		}
	}
	return executeErr
}

// Take the spec given in the job, and figure out which services are
// in question based on the running services and those defined in the
// repo. Fill in the release results along the way.
func selectServices(rc *ReleaseContext, spec *flux.ReleaseSpec, results flux.ReleaseResult, logStatus statusFn) ([]*ServiceUpdate, error) {
	conf, err := rc.Instance.GetConfig()
	if err != nil {
		return nil, err
	}
	lockedSet := LockedServices(conf)

	excludedSet := flux.ServiceIDSet{}
	excludedSet.Add(spec.Excludes)

	// For backwards-compatibility, there's two fields: ServiceSpec
	// and ServiceSpecs. An entry in ServiceSpec takes precedence.
	switch spec.ServiceSpec {
	case flux.ServiceSpec(""):
		ids := []flux.ServiceID{}
		for _, s := range spec.ServiceSpecs {
			if s == flux.ServiceSpecAll {
				return rc.SelectServices(nil, lockedSet, excludedSet, results, logStatus)
			}
			id, err := flux.ParseServiceID(string(s))
			if err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
		return rc.SelectServices(ids, lockedSet, excludedSet, results, logStatus)
	case flux.ServiceSpecAll:
		return rc.SelectServices(nil, lockedSet, excludedSet, results, logStatus)
	default:
		id, err := flux.ParseServiceID(string(spec.ServiceSpec))
		if err != nil {
			return nil, err
		}
		return rc.SelectServices([]flux.ServiceID{id}, lockedSet, excludedSet, results, logStatus)
	}
}

// Find all the image updates that should be performed, and do
// replacements. (For a dry-run, we don't strictly need to do the
// replacements, since we won't be committing any changes back;
// however we do want to see if we *can* do the replacements, because
// if not, it indicates there's likely some problem with the running
// system vs the definitions given in the repo.)
func calculateImageUpdates(inst *instance.Instance, candidates []*ServiceUpdate, spec *flux.ReleaseSpec, results flux.ReleaseResult, logStatus statusFn) ([]*ServiceUpdate, error) {
	// Compile an `ImageMap` of all relevant images
	var images instance.ImageMap
	var err error

	switch spec.ImageSpec {
	case flux.ImageSpecNone:
		images = instance.ImageMap{}
	case flux.ImageSpecLatest:
		images, err = CollectAvailableImages(inst, candidates)
	default:
		var image flux.ImageID
		image, err = spec.ImageSpec.AsID()
		if err == nil {
			images, err = inst.ExactImages([]flux.ImageID{image})
		}
	}

	if err != nil {
		return nil, err
	}

	// Look through all the services' containers to see which have an
	// image that could be updated.
	var updates []*ServiceUpdate
	for _, update := range candidates {
		containers, err := update.Service.ContainersOrError()
		if err != nil {
			logStatus("Failing service %s: %s", update.ServiceID, err.Error())
			results[update.ServiceID] = flux.ServiceResult{
				Status: flux.ReleaseStatusFailed,
				Error:  err.Error(),
			}
			continue
		}

		// If at least one container used an image in question, we say
		// we're skipping it rather than ignoring it. This is mainly
		// for the purpose of filtering the output.
		ignoredOrSkipped := flux.ReleaseStatusIgnored
		var containerUpdates []flux.ContainerUpdate

		for _, container := range containers {
			currentImageID, err := flux.ParseImageID(container.Image)
			if err != nil {
				// We may hope never to find a malformed image ID, but
				// anything is possible.
				return nil, err
			}

			latestImage := images.LatestImage(currentImageID.Repository())
			if latestImage == nil {
				continue
			}

			if currentImageID == latestImage.ID {
				ignoredOrSkipped = flux.ReleaseStatusSkipped
				continue
			}

			update.ManifestBytes, err = kubernetes.UpdatePodController(update.ManifestBytes, latestImage.ID, ioutil.Discard)
			if err != nil {
				return nil, err
			}

			logStatus("Will update %s container %s: %s -> %s", update.ServiceID, container.Name, currentImageID, latestImage.ID.Tag)
			containerUpdates = append(containerUpdates, flux.ContainerUpdate{
				Container: container.Name,
				Current:   currentImageID,
				Target:    latestImage.ID,
			})
		}

		switch {
		case len(containerUpdates) > 0:
			update.Updates = containerUpdates
			updates = append(updates, update)
			results[update.ServiceID] = flux.ServiceResult{
				Status:       flux.ReleaseStatusPending,
				PerContainer: containerUpdates,
			}
		case ignoredOrSkipped == flux.ReleaseStatusSkipped:
			logStatus("Skipping service %s, images are up to date", update.ServiceID)
			results[update.ServiceID] = flux.ServiceResult{
				Status: flux.ReleaseStatusSkipped,
				Error:  "image(s) up to date",
			}
		case ignoredOrSkipped == flux.ReleaseStatusIgnored:
			logStatus("Ignoring service %s, does not use image(s) in question", update.ServiceID)
			results[update.ServiceID] = flux.ServiceResult{
				Status: flux.ReleaseStatusIgnored,
				Error:  "does not use image(s)",
			}
		}
	}

	return updates, nil
}

func commitMessageFromReleaseSpec(spec *flux.ReleaseSpec) string {
	image := strings.Trim(spec.ImageSpec.String(), "<>")
	var services []string
	for _, s := range spec.ServiceSpecs {
		services = append(services, strings.Trim(s.String(), "<>"))
	}
	return fmt.Sprintf("Release %s to %s", image, strings.Join(services, ", "))
}
