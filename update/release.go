package update

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/go-kit/kit/log"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/registry"
)

const (
	ResourceSpecAll = ResourceSpec("<all>")
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
	ReleaseKindExecute ReleaseKind = "execute"
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
	SelectServices(Result, []ControllerFilter, []ControllerFilter) ([]*ControllerUpdate, error)
	ServicesWithPolicies() (policy.ResourceMap, error)
	Registry() registry.Registry
	Manifests() cluster.Manifests
}

// NB: these get sent from fluxctl, so we have to maintain the json format of
// this. Eugh.
type ReleaseSpec struct {
	ServiceSpecs []ResourceSpec
	ImageSpec    ImageSpec
	Kind         ReleaseKind
	Excludes     []flux.ResourceID
	Force        bool
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

func (s ReleaseSpec) CalculateRelease(rc ReleaseContext, logger log.Logger) ([]*ControllerUpdate, Result, error) {
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

func (s ReleaseSpec) CommitMessage(result Result) string {
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
func (s ReleaseSpec) selectServices(rc ReleaseContext, results Result) ([]*ControllerUpdate, error) {
	// Build list of filters
	prefilters, postfilters, err := s.filters(rc)
	if err != nil {
		return nil, err
	}
	// Find and filter services
	return rc.SelectServices(results, prefilters, postfilters)
}

func (s ReleaseSpec) filters(rc ReleaseContext) ([]ControllerFilter, []ControllerFilter, error) {
	var prefilters, postfilters []ControllerFilter

	ids := []flux.ResourceID{}
	for _, s := range s.ServiceSpecs {
		if s == ResourceSpecAll {
			// "<all>" Overrides any other filters
			ids = []flux.ResourceID{}
			break
		}
		id, err := flux.ParseResourceID(string(s))
		if err != nil {
			return nil, nil, err
		}
		ids = append(ids, id)
	}
	if len(ids) > 0 {
		prefilters = append(prefilters, &IncludeFilter{ids})
	}

	// Exclude filter
	if len(s.Excludes) > 0 {
		prefilters = append(prefilters, &ExcludeFilter{s.Excludes})
	}

	// Image filter
	if s.ImageSpec != ImageSpecLatest {
		id, err := image.ParseRef(s.ImageSpec.String())
		if err != nil {
			return nil, nil, err
		}
		postfilters = append(postfilters, &SpecificImageFilter{id})
	}

	// Locked filter
	services, err := rc.ServicesWithPolicies()
	if err != nil {
		return nil, nil, err
	}
	lockedSet := services.OnlyWithPolicy(policy.Locked)
	postfilters = append(postfilters, &LockedFilter{lockedSet.ToSlice()})

	return prefilters, postfilters, nil
}

func (s ReleaseSpec) markSkipped(results Result) {
	for _, v := range s.ServiceSpecs {
		if v == ResourceSpecAll {
			continue
		}
		id, err := v.AsID()
		if err != nil {
			continue
		}
		if _, ok := results[id]; !ok {
			results[id] = ControllerResult{
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
func (s ReleaseSpec) calculateImageUpdates(rc ReleaseContext, candidates []*ControllerUpdate, results Result, logger log.Logger) ([]*ControllerUpdate, error) {
	// Compile an `ImageRepos` of all relevant images
	var imageRepos ImageRepos
	var singleRepo image.CanonicalName
	var err error

	switch s.ImageSpec {
	case ImageSpecLatest:
		imageRepos, err = fetchUpdatableImageRepos(rc.Registry(), candidates, logger)
	default:
		var ref image.Ref
		ref, err = s.ImageSpec.AsRef()
		if err == nil {
			singleRepo = ref.CanonicalName()
			imageRepos, err = exactImageRepos(rc.Registry(), []image.Ref{ref})
		}
	}

	if err != nil {
		return nil, err
	}

	// Look through all the services' containers to see which have an
	// image that could be updated.
	var updates []*ControllerUpdate
	for _, u := range candidates {
		containers, err := u.Controller.ContainersOrError()
		if err != nil {
			results[u.ResourceID] = ControllerResult{
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
			currentImageID := container.Image

			filteredImages := imageRepos.GetRepoImages(currentImageID.Name).Filter(policy.PatternAll)
			latestImage, ok := filteredImages.Latest()
			if !ok {
				if currentImageID.CanonicalName() != singleRepo {
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

			// We want to update the image with respect to the form it
			// appears in the manifest, whereas what we have is the
			// canonical form.
			newImageID := currentImageID.WithNewTag(latestImage.ID.Tag)
			containerUpdates = append(containerUpdates, ContainerUpdate{
				Container: container.Name,
				Current:   currentImageID,
				Target:    newImageID,
			})
		}

		switch {
		case len(containerUpdates) > 0:
			u.Updates = containerUpdates
			updates = append(updates, u)
			results[u.ResourceID] = ControllerResult{
				Status:       ReleaseStatusSuccess,
				PerContainer: containerUpdates,
			}
		case ignoredOrSkipped == ReleaseStatusSkipped:
			results[u.ResourceID] = ControllerResult{
				Status: ReleaseStatusSkipped,
				Error:  ImageUpToDate,
			}
		case ignoredOrSkipped == ReleaseStatusIgnored:
			results[u.ResourceID] = ControllerResult{
				Status: ReleaseStatusIgnored,
				Error:  DoesNotUseImage,
			}
		case ignoredOrSkipped == ReleaseStatusUnknown:
			results[u.ResourceID] = ControllerResult{
				Status: ReleaseStatusSkipped,
				Error:  ImageNotFound,
			}
		}
	}

	return updates, nil
}

type ResourceSpec string // ResourceID or "<all>"

func ParseResourceSpec(s string) (ResourceSpec, error) {
	if s == string(ResourceSpecAll) {
		return ResourceSpecAll, nil
	}
	id, err := flux.ParseResourceID(s)
	if err != nil {
		return "", errors.Wrap(err, "invalid service spec")
	}
	return ResourceSpec(id.String()), nil
}

func MakeResourceSpec(id flux.ResourceID) ResourceSpec {
	return ResourceSpec(id.String())
}

func (s ResourceSpec) AsID() (flux.ResourceID, error) {
	return flux.ParseResourceID(string(s))
}

func (s ResourceSpec) String() string {
	return string(s)
}

// ImageSpec is an ImageID, or "<all latest>" (update all containers
// to the latest available).
type ImageSpec string

func ParseImageSpec(s string) (ImageSpec, error) {
	if s == string(ImageSpecLatest) {
		return ImageSpecLatest, nil
	}

	id, err := image.ParseRef(s)
	if err != nil {
		return "", err
	}
	if id.Tag == "" {
		return "", errors.Wrap(image.ErrInvalidImageID, "blank tag (if you want latest, explicitly state the tag :latest)")
	}
	return ImageSpec(id.String()), err
}

func (s ImageSpec) String() string {
	return string(s)
}

func (s ImageSpec) AsRef() (image.Ref, error) {
	return image.ParseRef(s.String())
}

func ImageSpecFromRef(id image.Ref) ImageSpec {
	return ImageSpec(id.String())
}
