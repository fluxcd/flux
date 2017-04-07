package release

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-kit/kit/metrics"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	fluxmetrics "github.com/weaveworks/flux/metrics"
	"github.com/weaveworks/flux/platform"
)

type ServiceUpdate struct {
	ServiceID     flux.ServiceID
	Service       platform.Service
	ManifestPath  string
	ManifestBytes []byte
	Updates       []flux.ContainerUpdate
}

func Release(daemon platform.Daemon, spec flux.ReleaseSpec) (results flux.ReleaseResult, err error) {
	started := time.Now()
	defer func(start time.Time) {
		releaseDuration.With(
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
			fluxmetrics.LabelReleaseType, flux.ReleaseSpec(spec).ReleaseType(),
			fluxmetrics.LabelReleaseKind, string(spec.Kind),
		).Observe(time.Since(started).Seconds())
	}(started)

	// We time each stage of this process, and expose as metrics.
	var timer *metrics.Timer

	// Preparation: we always need the repository
	rc := NewReleaseContext(daemon.Cluster, daemon.Repo)
	defer rc.Clean()
	timer = NewStageTimer("clone_repository")
	if err = rc.CloneRepo(); err != nil {
		return nil, err
	}
	timer.ObserveDuration()

	// From here in, we collect the results of the calculations.
	results = flux.ReleaseResult{}

	// Figure out the services involved.
	timer = NewStageTimer("select_services")
	var updates []*ServiceUpdate
	updates, err = selectServices(rc, &spec, results)
	timer.ObserveDuration()
	if err != nil {
		return nil, err
	}

	// Look up images, and calculate updates, if we've been asked to
	if spec.ImageSpec != flux.ImageSpecNone {
		timer = NewStageTimer("lookup_images")
		// Figure out how the services are to be updated.
		updates, err = calculateImageUpdates(daemon, updates, &spec, results)
		timer.ObserveDuration()
		if err != nil {
			return nil, err
		}
	}

	// At this point we may have filtered the updates we can do down
	// to nothing. Check and exit early if so.
	if len(updates) == 0 {
		return results, nil
	}

	// If it's a dry run, we're done.
	if spec.Kind == flux.ReleaseKindPlan {
		return results, nil
	}

	if spec.ImageSpec != flux.ImageSpecNone {
		timer = NewStageTimer("push_changes")
		err = rc.PushChanges(updates, &spec)
		timer.ObserveDuration()
		if err != nil {
			return nil, err
		}
	}

	status := flux.ReleaseStatusSuccess
	if err != nil {
		status = flux.ReleaseStatusFailed
	}
	release := flux.Release{
		StartedAt: started,
		// TODO: fetch the job and look this up so it matches
		// (which must be done after completing the job)
		EndedAt: time.Now().UTC(),
		Done:    true,
		Status:  status,
		// %%%FIXME reinstate the log, if it's useful
		//		Log:      logged,

		// %%% FIXME where does this come from? Redesign
		//		Cause:  job.Params.(jobs.ReleaseJobParams).Cause,
		Spec:   spec,
		Result: results,
	}

	// Log the event into the history
	timer = NewStageTimer("log_event")
	err = logEvent(daemon, err, release)
	timer.ObserveDuration()

	return results, err
}

// `logEvent` expects the result of applying updates, and records an event in
// the history about the release taking place. It returns the origin error if
// that was non-nil, otherwise the result of the attempted logging.
func logEvent(d platform.Daemon, executeErr error, release flux.Release) error {
	errorMessage := ""
	logLevel := flux.LogLevelInfo
	if executeErr != nil {
		errorMessage = executeErr.Error()
		logLevel = flux.LogLevelError
	}

	var serviceIDs []flux.ServiceID
	for k, v := range release.Result {
		if v.Status != flux.ReleaseStatusIgnored {
			serviceIDs = append(serviceIDs, flux.ServiceID(k))
		}
	}

	err := d.LogEvent(flux.Event{
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

// Take the spec given in the job, and figure out which services are
// in question based on the running services and those defined in the
// repo. Fill in the release results along the way.
func selectServices(rc *ReleaseContext, spec *flux.ReleaseSpec, results flux.ReleaseResult) ([]*ServiceUpdate, error) {
	// Build list of filters
	filtList, err := filters(spec, rc)
	if err != nil {
		return nil, err
	}
	// Find and filter services
	return rc.SelectServices(
		results,
		filtList...,
	)
}

// filters converts a ReleaseSpec (and Lock config) into ServiceFilters
func filters(spec *flux.ReleaseSpec, rc *ReleaseContext) ([]ServiceFilter, error) {
	// Image filter
	var filtList []ServiceFilter
	if spec.ImageSpec != flux.ImageSpecNone && spec.ImageSpec != flux.ImageSpecLatest {
		id, err := flux.ParseImageID(spec.ImageSpec.String())
		if err != nil {
			return nil, err
		}
		imgFilt := &SpecificImageFilter{id}
		filtList = append(filtList, imgFilt)
	}

	// Service filter
	ids := []flux.ServiceID{}
	for _, s := range spec.ServiceSpecs {
		if s == flux.ServiceSpecAll {
			ids = []flux.ServiceID{} // "<all>" Overrides any other filters
			break
		}
		id, err := flux.ParseServiceID(string(s))
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if len(ids) > 0 {
		incFilt := &IncludeFilter{ids}
		filtList = append(filtList, incFilt)
	}

	// Exclude filter
	if len(spec.Excludes) > 0 {
		exFilt := &ExcludeFilter{spec.Excludes}
		filtList = append(filtList, exFilt)
	}

	// Locked filter
	lockedSet := rc.LockedServices()
	lockFilt := &LockedFilter{lockedSet.ToSlice()}
	filtList = append(filtList, lockFilt)
	return filtList, nil
}

// Find all the image updates that should be performed, and do
// replacements. (For a dry-run, we don't strictly need to do the
// replacements, since we won't be committing any changes back;
// however we do want to see if we *can* do the replacements, because
// if not, it indicates there's likely some problem with the running
// system vs the definitions given in the repo.)
func calculateImageUpdates(daemon platform.Daemon, candidates []*ServiceUpdate, spec *flux.ReleaseSpec, results flux.ReleaseResult) ([]*ServiceUpdate, error) {
	// Compile an `ImageMap` of all relevant images
	var images platform.ImageMap
	var err error

	switch spec.ImageSpec {
	case flux.ImageSpecNone:
		images = platform.ImageMap{}
	case flux.ImageSpecLatest:
		images, err = CollectAvailableImages(daemon.Registry, candidates)
	default:
		var image flux.ImageID
		image, err = spec.ImageSpec.AsID()
		if err == nil {
			images, err = platform.ExactImages(daemon.Registry, []flux.ImageID{image})
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
				ignoredOrSkipped = flux.ReleaseStatusUnknown
				continue
			}

			if currentImageID == latestImage.ID {
				ignoredOrSkipped = flux.ReleaseStatusSkipped
				continue
			}

			update.ManifestBytes, err = daemon.Cluster.UpdateDefinition(update.ManifestBytes, latestImage.ID)
			if err != nil {
				return nil, err
			}

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
			results[update.ServiceID] = flux.ServiceResult{
				Status: flux.ReleaseStatusSkipped,
				Error:  ImageUpToDate,
			}
		case ignoredOrSkipped == flux.ReleaseStatusIgnored:
			results[update.ServiceID] = flux.ServiceResult{
				Status: flux.ReleaseStatusIgnored,
				Error:  "does not use image(s)",
			}
		case ignoredOrSkipped == flux.ReleaseStatusUnknown:
			results[update.ServiceID] = flux.ServiceResult{
				Status: flux.ReleaseStatusSkipped,
				Error:  ImageNotFound,
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
