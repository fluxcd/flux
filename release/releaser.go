package release

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-kit/kit/log"

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

func Release(rc *ReleaseContext, spec update.ReleaseSpec, logger log.Logger) (results update.Result, err error) {
	started := time.Now()
	defer func(start time.Time) {
		releaseDuration.With(
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
			fluxmetrics.LabelReleaseType, update.ReleaseSpec(spec).ReleaseType(),
			fluxmetrics.LabelReleaseKind, string(spec.Kind),
		).Observe(time.Since(started).Seconds())
	}(started)

	logger = log.NewContext(logger).With("type", "release")

	updates, results, err := CalculateRelease(rc, spec, logger)
	if err != nil {
		return nil, err
	}

	err = ApplyChanges(rc, updates, logger)
	return results, err

}

func CalculateRelease(rc *ReleaseContext, spec update.ReleaseSpec, logger log.Logger) ([]*ServiceUpdate, update.Result, error) {
	results := update.Result{}
	timer := NewStageTimer("select_services")
	updates, err := selectServices(rc, &spec, results)
	timer.ObserveDuration()
	if err != nil {
		return nil, nil, err
	}
	markSkipped(spec, results)

	timer = NewStageTimer("lookup_images")
	updates, err = calculateImageUpdates(rc, updates, &spec, results, logger)
	timer.ObserveDuration()
	if err != nil {
		return nil, nil, err
	}
	return updates, results, nil
}

func markSkipped(spec update.ReleaseSpec, results update.Result) {
	for _, v := range spec.ServiceSpecs {
		if v == update.ServiceSpecAll {
			continue
		}
		id, err := v.AsID()
		if err != nil {
			continue
		}
		if _, ok := results[id]; !ok {
			results[id] = update.ServiceResult{
				Status: update.ReleaseStatusSkipped,
				Error:  NotInRepo,
			}
		}
	}
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
	if spec.ImageSpec != update.ImageSpecLatest {
		id, err := flux.ParseImageID(spec.ImageSpec.String())
		if err != nil {
			return nil, err
		}
		filtList = append(filtList, &SpecificImageFilter{id})
	}

	// Service filter
	ids := []flux.ServiceID{}
specs:
	for _, s := range spec.ServiceSpecs {
		switch s {
		case update.ServiceSpecAll:
			// "<all>" Overrides any other filters
			ids = []flux.ServiceID{}
			break specs
		case update.ServiceSpecAutomated:
			// "<automated>" Overrides any other filters
			automated, err := rc.ServicesWithPolicy(policy.Automated)
			if err != nil {
				return nil, err
			}
			ids = automated.ToSlice()
			break specs
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
func calculateImageUpdates(rc *ReleaseContext, candidates []*ServiceUpdate, spec *update.ReleaseSpec, results update.Result, logger log.Logger) ([]*ServiceUpdate, error) {
	// Compile an `ImageMap` of all relevant images
	var images ImageMap
	var repo string
	var err error

	switch spec.ImageSpec {
	case update.ImageSpecLatest:
		images, err = CollectUpdateImages(rc.Registry, candidates)
	default:
		var image flux.ImageID
		image, err = spec.ImageSpec.AsID()
		if err == nil {
			repo = image.Repository()
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

		// TODO #260 revisit this?
		// Filter container name if it's present in spec
		if fc, err := filterContainers(containers, spec); err == nil {
			containers = fc
		} else {
			logger.Log("err", err)
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

			latestImage := images.LatestImage(currentImageID.Repository(), "*")
			if latestImage == nil {
				if currentImageID.Repository() != repo {
					ignoredOrSkipped = update.ReleaseStatusIgnored
				} else {
					ignoredOrSkipped = update.ReleaseStatusUnknown
				}
				continue
			}

			if currentImageID == latestImage.ID {
				ignoredOrSkipped = update.ReleaseStatusSkipped
				continue
			}

			u.ManifestBytes, err = rc.Manifests.UpdateDefinition(u.ManifestBytes, latestImage.ID)
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
				Status:       update.ReleaseStatusSuccess,
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
				Error:  DoesNotUseImage,
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

func ApplyChanges(rc *ReleaseContext, updates []*ServiceUpdate, logger log.Logger) error {
	logger.Log("updates", len(updates))
	if len(updates) == 0 {
		logger.Log("exit", "no images to update for services given")
		return nil
	} else {
		l := log.NewContext(logger).With("msg", "applying changes")
		for _, u := range updates {
			l.Log("changes", fmt.Sprintf("%#v", *u))
		}
	}

	timer := NewStageTimer("push_changes")
	err := rc.WriteUpdates(updates)
	timer.ObserveDuration()
	return err
}

func filterContainers(containers []cluster.Container, spec *update.ReleaseSpec) ([]cluster.Container, error) {
	// Name unspecified, don't filter
	if spec.ContainerName == "" {
		return containers, nil
	}
	// Make sure the specified container exists
	for _, c := range containers {
		if c.Name == spec.ContainerName {
			return []cluster.Container{c}, nil
		}
	}
	return nil, fmt.Errorf("no container with name %s", spec.ContainerName)
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
