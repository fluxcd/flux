package update

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/go-kit/kit/log"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/registry"
)

const (
	ServiceSpecAll  = ServiceSpec("<all>")
	ImageSpecLatest = ImageSpec("<all latest>")
)

var (
	ErrInvalidReleaseKind = errors.New("invalid release kind")
)

// ReleaseKind says whether a release is to be planned only, or planned then executed
type ReleaseKind string
type ReleaseType string

const (
	ReleaseKindPlan    ReleaseKind = "plan"
	ReleaseKindExecute             = "execute"
)

func ParseReleaseKind(s string) (ReleaseKind, error) {
	switch s {
	case string(ReleaseKindPlan):
		return ReleaseKindPlan, nil
	case string(ReleaseKindExecute):
		return ReleaseKindExecute, nil
	default:
		return "", ErrInvalidReleaseKind
	}
}

const UserAutomated = "<automated>"

type ReleaseContext interface {
	SelectServices(Result, ...ServiceFilter) ([]*ServiceUpdate, error)
	ServicesWithPolicy(policy.Policy) (policy.ServiceMap, error)
	Registry() registry.Registry
	Manifests() cluster.Manifests
}

// NB: these get sent from fluxctl, so we have to maintain the json format of
// this. Eugh.
type ReleaseSpec struct {
	ServiceSpecs []ServiceSpec
	ImageSpec    ImageSpec
	Kind         ReleaseKind
	Excludes     []flux.ServiceID
}

// ReleaseType gives a one-word description of the release, mainly
// useful for labelling metrics or log messages.
func (s ReleaseSpec) ReleaseType() ReleaseType {
	switch {
	case s.ImageSpec == ImageSpecLatest:
		return "latest_images"
	default:
		return "specific_image"
	}
}

func (s ReleaseSpec) CalculateRelease(rc ReleaseContext, logger log.Logger) ([]*ServiceUpdate, Result, error) {
	results := Result{}
	timer := NewStageTimer("select_services")
	updates, err := s.selectServices(rc, results)
	timer.ObserveDuration()
	if err != nil {
		return nil, nil, err
	}
	s.markSkipped(results)

	timer = NewStageTimer("lookup_images")
	updates, err = s.calculateImageUpdates(rc, updates, results, logger)
	timer.ObserveDuration()
	if err != nil {
		return nil, nil, err
	}
	return updates, results, nil
}

func (s ReleaseSpec) ReleaseKind() ReleaseKind {
	return s.Kind
}

func (s ReleaseSpec) CommitMessage() string {
	image := strings.Trim(s.ImageSpec.String(), "<>")
	var services []string
	for _, spec := range s.ServiceSpecs {
		services = append(services, strings.Trim(spec.String(), "<>"))
	}
	return fmt.Sprintf("Release %s to %s", image, strings.Join(services, ", "))
}

// Take the spec given in the job, and figure out which services are
// in question based on the running services and those defined in the
// repo. Fill in the release results along the way.
func (s ReleaseSpec) selectServices(rc ReleaseContext, results Result) ([]*ServiceUpdate, error) {
	// Build list of filters
	filtList, err := s.filters(rc)
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
func (s ReleaseSpec) filters(rc ReleaseContext) ([]ServiceFilter, error) {
	// Image filter
	var filtList []ServiceFilter
	if s.ImageSpec != ImageSpecLatest {
		id, err := flux.ParseImageID(s.ImageSpec.String())
		if err != nil {
			return nil, err
		}
		filtList = append(filtList, &SpecificImageFilter{id})
	}

	// Service filter
	ids := []flux.ServiceID{}
	for _, s := range s.ServiceSpecs {
		if s == ServiceSpecAll {
			// "<all>" Overrides any other filters
			ids = []flux.ServiceID{}
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
	if len(s.Excludes) > 0 {
		filtList = append(filtList, &ExcludeFilter{s.Excludes})
	}

	// Locked filter
	lockedSet, err := rc.ServicesWithPolicy(policy.Locked)
	if err != nil {
		return nil, err
	}
	filtList = append(filtList, &LockedFilter{lockedSet.ToSlice()})
	return filtList, nil
}

func (s ReleaseSpec) markSkipped(results Result) {
	for _, v := range s.ServiceSpecs {
		if v == ServiceSpecAll {
			continue
		}
		id, err := v.AsID()
		if err != nil {
			continue
		}
		if _, ok := results[id]; !ok {
			results[id] = ServiceResult{
				Status: ReleaseStatusSkipped,
				Error:  NotInRepo,
			}
		}
	}
}

// Find all the image updates that should be performed, and do
// replacements. (For a dry-run, we don't strictly need to do the
// replacements, since we won't be committing any changes back;
// however we do want to see if we *can* do the replacements, because
// if not, it indicates there's likely some problem with the running
// system vs the definitions given in the repo.)
func (s ReleaseSpec) calculateImageUpdates(rc ReleaseContext, candidates []*ServiceUpdate, results Result, logger log.Logger) ([]*ServiceUpdate, error) {
	// Compile an `ImageMap` of all relevant images
	var images ImageMap
	var repo string
	var err error

	switch s.ImageSpec {
	case ImageSpecLatest:
		images, err = collectUpdateImages(rc.Registry(), candidates)
	default:
		var image flux.ImageID
		image, err = s.ImageSpec.AsID()
		if err == nil {
			repo = image.Repository()
			images, err = exactImages(rc.Registry(), []flux.ImageID{image})
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
			results[u.ServiceID] = ServiceResult{
				Status: ReleaseStatusFailed,
				Error:  err.Error(),
			}
			continue
		}

		// If at least one container used an image in question, we say
		// we're skipping it rather than ignoring it. This is mainly
		// for the purpose of filtering the output.
		ignoredOrSkipped := ReleaseStatusIgnored
		var containerUpdates []ContainerUpdate

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
					ignoredOrSkipped = ReleaseStatusIgnored
				} else {
					ignoredOrSkipped = ReleaseStatusUnknown
				}
				continue
			}

			if currentImageID == latestImage.ID {
				ignoredOrSkipped = ReleaseStatusSkipped
				continue
			}

			u.ManifestBytes, err = rc.Manifests().UpdateDefinition(u.ManifestBytes, container.Name, latestImage.ID)
			if err != nil {
				return nil, err
			}

			containerUpdates = append(containerUpdates, ContainerUpdate{
				Container: container.Name,
				Current:   currentImageID,
				Target:    latestImage.ID,
			})
		}

		switch {
		case len(containerUpdates) > 0:
			u.Updates = containerUpdates
			updates = append(updates, u)
			results[u.ServiceID] = ServiceResult{
				Status:       ReleaseStatusSuccess,
				PerContainer: containerUpdates,
			}
		case ignoredOrSkipped == ReleaseStatusSkipped:
			results[u.ServiceID] = ServiceResult{
				Status: ReleaseStatusSkipped,
				Error:  ImageUpToDate,
			}
		case ignoredOrSkipped == ReleaseStatusIgnored:
			results[u.ServiceID] = ServiceResult{
				Status: ReleaseStatusIgnored,
				Error:  DoesNotUseImage,
			}
		case ignoredOrSkipped == ReleaseStatusUnknown:
			results[u.ServiceID] = ServiceResult{
				Status: ReleaseStatusSkipped,
				Error:  ImageNotFound,
			}
		}
	}

	return updates, nil
}

type ServiceSpec string // ServiceID or "<all>"

func ParseServiceSpec(s string) (ServiceSpec, error) {
	if s == string(ServiceSpecAll) {
		return ServiceSpecAll, nil
	}
	id, err := flux.ParseServiceID(s)
	if err != nil {
		return "", errors.Wrap(err, "invalid service spec")
	}
	return ServiceSpec(id), nil
}

func (s ServiceSpec) AsID() (flux.ServiceID, error) {
	return flux.ParseServiceID(string(s))
}

func (s ServiceSpec) String() string {
	return string(s)
}

// ImageSpec is an ImageID, or "<all latest>" (update all containers
// to the latest available), or "<no updates>" (do not update any
// images)
type ImageSpec string

func ParseImageSpec(s string) (ImageSpec, error) {
	if s == string(ImageSpecLatest) {
		return ImageSpec(s), nil
	}

	parts := strings.Split(s, ":")
	if len(parts) != 2 || parts[1] == "" {
		return "", errors.Wrap(flux.ErrInvalidImageID, "blank tag (if you want latest, explicitly state the tag :latest)")
	}

	id, err := flux.ParseImageID(s)
	return ImageSpec(id.String()), err
}

func (s ImageSpec) String() string {
	return string(s)
}

func (s ImageSpec) AsID() (flux.ImageID, error) {
	return flux.ParseImageID(s.String())
}

func ImageSpecFromID(id flux.ImageID) ImageSpec {
	return ImageSpec(id.String())
}
