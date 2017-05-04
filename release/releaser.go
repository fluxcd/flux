package release

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-kit/kit/metrics"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	fluxmetrics "github.com/weaveworks/flux/metrics"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/registry"
	"github.com/weaveworks/flux/update"
)

type ServiceUpdate struct {
	ServiceID     flux.ServiceID
	Service       cluster.Service
	ManifestPath  string
	ManifestBytes []byte
	Updates       []update.ContainerUpdate
}

func Release(rc *ReleaseContext, spec update.ReleaseSpec) (commitRef string, results update.Result, err error) {
	started := time.Now()
	defer func(start time.Time) {
		releaseDuration.With(
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
			fluxmetrics.LabelReleaseType, update.ReleaseSpec(spec).ReleaseType(),
			fluxmetrics.LabelReleaseKind, string(spec.Kind),
		).Observe(time.Since(started).Seconds())
	}(started)

	// We time each stage of this process, and expose as metrics.
	var timer *metrics.Timer

	// FIXME pull from the repository? Or rely on something else to do that.
	// ALSO: clean up in the result of failure, afterwards

	// From here in, we collect the results of the calculations.
	results = update.Result{}

	// Figure out the services involved.
	timer = NewStageTimer("select_services")
	var updates []*ServiceUpdate
	updates, err = selectServices(rc, &spec, results)
	timer.ObserveDuration()
	if err != nil {
		return "", nil, err
	}

	// Look up images, and calculate updates, if we've been asked to
	if spec.ImageSpec != update.ImageSpecNone {
		timer = NewStageTimer("lookup_images")
		// Figure out how the services are to be updated.
		updates, err = calculateImageUpdates(rc, updates, &spec, results)
		timer.ObserveDuration()
		if err != nil {
			return "", nil, err
		}
	}

	// At this point we may have filtered the updates we can do down
	// to nothing. Check and exit early if so.
	if len(updates) == 0 {
		return "", results, nil
	}

	// If it's a dry run, we're done.
	if spec.Kind == update.ReleaseKindPlan {
		return "", results, nil
	}

	if spec.ImageSpec != update.ImageSpecNone {
		timer = NewStageTimer("push_changes")
		err = rc.PushChanges(updates, &spec, results)
		timer.ObserveDuration()
		if err != nil {
			return "", nil, err
		}
	}

	revision, err := rc.HeadRevision()
	return revision, results, err
}

// Take the spec given in the job, and figure out which services are
// in question based on the running services and those defined in the
// repo. Fill in the release results along the way.
func selectServices(rc *ReleaseContext, spec *update.ReleaseSpec, results update.Result) ([]*ServiceUpdate, error) {
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
func filters(spec *update.ReleaseSpec, rc *ReleaseContext) ([]ServiceFilter, error) {
	// Image filter
	var filtList []ServiceFilter
	if spec.ImageSpec != update.ImageSpecNone && spec.ImageSpec != update.ImageSpecLatest {
		id, err := flux.ParseImageID(spec.ImageSpec.String())
		if err != nil {
			return nil, err
		}
		filtList = append(filtList, &SpecificImageFilter{id})
	}

	// Service filter
	ids := []flux.ServiceID{}
	for _, s := range spec.ServiceSpecs {
		switch s {
		case update.ServiceSpecAll:
			// "<all>" Overrides any other filters
			ids = []flux.ServiceID{}
			break
		case update.ServiceSpecAutomated:
			// "<automated>" Overrides any other filters
			automated, err := rc.ServicesWithPolicy(policy.Automated)
			if err != nil {
				return nil, err
			}
			ids = automated.ToSlice()
			break
		}
		id, err := flux.ParseServiceID(string(s))
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if len(ids) > 0 {
		filtList = append(filtList, &IncludeFilter{ids})
	}

	// Exclude filter
	if len(spec.Excludes) > 0 {
		filtList = append(filtList, &ExcludeFilter{spec.Excludes})
	}

	// Locked filter
	lockedSet, err := rc.ServicesWithPolicy(policy.Locked)
	if err != nil {
		return nil, err
	}
	filtList = append(filtList, &LockedFilter{lockedSet.ToSlice()})
	return filtList, nil
}

// Find all the image updates that should be performed, and do
// replacements. (For a dry-run, we don't strictly need to do the
// replacements, since we won't be committing any changes back;
// however we do want to see if we *can* do the replacements, because
// if not, it indicates there's likely some problem with the running
// system vs the definitions given in the repo.)
func calculateImageUpdates(rc *ReleaseContext, candidates []*ServiceUpdate, spec *update.ReleaseSpec, results update.Result) ([]*ServiceUpdate, error) {
	// Compile an `ImageMap` of all relevant images
	var images ImageMap
	var err error

	switch spec.ImageSpec {
	case update.ImageSpecNone:
		images = ImageMap{}
	case update.ImageSpecLatest:
		images, err = CollectUpdateImages(rc.Registry, candidates)
	default:
		var image flux.ImageID
		image, err = spec.ImageSpec.AsID()
		if err == nil {
			images, err = ExactImages(rc.Registry, []flux.ImageID{image})
		}
	}

	if err != nil {
		return nil, err
	}

	// Look through all the services' containers to see which have an
	// image that could be updated.
	var updates []*ServiceUpdate
	for _, u := range candidates {
		containers, err := u.Service.ContainersOrError()
		if err != nil {
			results[u.ServiceID] = update.ServiceResult{
				Status: update.ReleaseStatusFailed,
				Error:  err.Error(),
			}
			continue
		}

		// If at least one container used an image in question, we say
		// we're skipping it rather than ignoring it. This is mainly
		// for the purpose of filtering the output.
		ignoredOrSkipped := update.ReleaseStatusIgnored
		var containerUpdates []update.ContainerUpdate

		for _, container := range containers {
			currentImageID, err := flux.ParseImageID(container.Image)
			if err != nil {
				// We may hope never to find a malformed image ID, but
				// anything is possible.
				return nil, err
			}

			latestImage := images.LatestImage(currentImageID.Repository())
			if latestImage == nil {
				ignoredOrSkipped = update.ReleaseStatusUnknown
				continue
			}

			if currentImageID == latestImage.ID {
				ignoredOrSkipped = update.ReleaseStatusSkipped
				continue
			}

			u.ManifestBytes, err = rc.Cluster.UpdateDefinition(u.ManifestBytes, latestImage.ID)
			if err != nil {
				return nil, err
			}

			containerUpdates = append(containerUpdates, update.ContainerUpdate{
				Container: container.Name,
				Current:   currentImageID,
				Target:    latestImage.ID,
			})
		}

		switch {
		case len(containerUpdates) > 0:
			u.Updates = containerUpdates
			updates = append(updates, u)
			results[u.ServiceID] = update.ServiceResult{
				Status:       update.ReleaseStatusPending,
				PerContainer: containerUpdates,
			}
		case ignoredOrSkipped == update.ReleaseStatusSkipped:
			results[u.ServiceID] = update.ServiceResult{
				Status: update.ReleaseStatusSkipped,
				Error:  ImageUpToDate,
			}
		case ignoredOrSkipped == update.ReleaseStatusIgnored:
			results[u.ServiceID] = update.ServiceResult{
				Status: update.ReleaseStatusIgnored,
				Error:  "does not use image(s)",
			}
		case ignoredOrSkipped == update.ReleaseStatusUnknown:
			results[u.ServiceID] = update.ServiceResult{
				Status: update.ReleaseStatusSkipped,
				Error:  ImageNotFound,
			}
		}
	}

	return updates, nil
}

func commitMessageFromReleaseSpec(spec *update.ReleaseSpec) string {
	image := strings.Trim(spec.ImageSpec.String(), "<>")
	var services []string
	for _, s := range spec.ServiceSpecs {
		services = append(services, strings.Trim(s.String(), "<>"))
	}
	return fmt.Sprintf("Release %s to %s", image, strings.Join(services, ", "))
}

// CollectUpdateImages is a convenient shim to
// `CollectAvailableImages`.
func CollectUpdateImages(registry registry.Registry, updateable []*ServiceUpdate) (ImageMap, error) {
	var servicesToCheck []cluster.Service
	for _, update := range updateable {
		servicesToCheck = append(servicesToCheck, update.Service)
	}
	return CollectAvailableImages(registry, servicesToCheck)
}
