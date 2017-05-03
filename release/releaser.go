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
)

type ServiceUpdate struct {
	ServiceID     flux.ServiceID
	Service       cluster.Service
	ManifestPath  string
	ManifestBytes []byte
	Updates       []flux.ContainerUpdate
}

func Release(rc *ReleaseContext, spec flux.ReleaseSpec) (results flux.ReleaseResult, err error) {
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

	// FIXME pull from the repository? Or rely on something else to do that.
	// ALSO: clean up in the result of failure, afterwards

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
		updates, err = calculateImageUpdates(rc, updates, &spec, results)
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
		err = rc.PushChanges(updates, &spec, results)
		timer.ObserveDuration()
		if err != nil {
			return nil, err
		}
	}

	return results, err
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
		filtList = append(filtList, &SpecificImageFilter{id})
	}

	// Service filter
	ids := []flux.ServiceID{}
	for _, s := range spec.ServiceSpecs {
		switch s {
		case flux.ServiceSpecAll:
			// "<all>" Overrides any other filters
			ids = []flux.ServiceID{}
			break
		case flux.ServiceSpecAutomated:
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
func calculateImageUpdates(rc *ReleaseContext, candidates []*ServiceUpdate, spec *flux.ReleaseSpec, results flux.ReleaseResult) ([]*ServiceUpdate, error) {
	// Compile an `ImageMap` of all relevant images
	var images ImageMap
	var err error

	switch spec.ImageSpec {
	case flux.ImageSpecNone:
		images = ImageMap{}
	case flux.ImageSpecLatest:
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

			update.ManifestBytes, err = rc.Cluster.UpdateDefinition(update.ManifestBytes, latestImage.ID)
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

// CollectUpdateImages is a convenient shim to
// `CollectAvailableImages`.
func CollectUpdateImages(registry registry.Registry, updateable []*ServiceUpdate) (ImageMap, error) {
	var servicesToCheck []cluster.Service
	for _, update := range updateable {
		servicesToCheck = append(servicesToCheck, update.Service)
	}
	return CollectAvailableImages(registry, servicesToCheck)
}
