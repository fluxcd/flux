package update

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/policy"
	"github.com/fluxcd/flux/pkg/registry"
	"github.com/fluxcd/flux/pkg/resource"
	"github.com/go-kit/kit/log"
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
	SelectWorkloads(context.Context, Result, []WorkloadFilter, []WorkloadFilter) ([]*WorkloadUpdate, error)
	Registry() registry.Registry
}

// NB: these get sent from fluxctl, so we have to maintain the json format of
// this. Eugh.
type ReleaseImageSpec struct {
	ServiceSpecs []ResourceSpec // TODO: rename to WorkloadSpecs after adding versioning
	ImageSpec    ImageSpec
	Kind         ReleaseKind
	Excludes     []resource.ID
	Force        bool
}

// ReleaseType gives a one-word description of the release, mainly
// useful for labelling metrics or log messages.
func (s ReleaseImageSpec) ReleaseType() ReleaseType {
	switch {
	case s.ImageSpec == ImageSpecLatest:
		return "latest_images"
	default:
		return "specific_image"
	}
}

func (s ReleaseImageSpec) CalculateRelease(ctx context.Context, rc ReleaseContext, logger log.Logger) ([]*WorkloadUpdate, Result, error) {
	results := Result{}
	timer := NewStageTimer("select_workloads")
	updates, err := s.selectWorkloads(ctx, rc, results)
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

func (s ReleaseImageSpec) ReleaseKind() ReleaseKind {
	return s.Kind
}

func (s ReleaseImageSpec) CommitMessage(result Result) string {
	image := strings.Trim(s.ImageSpec.String(), "<>")
	var workloads []string
	for _, spec := range s.ServiceSpecs {
		workloads = append(workloads, strings.Trim(spec.String(), "<>"))
	}
	return fmt.Sprintf("Release %s to %s", image, strings.Join(workloads, ", "))
}

// Take the spec given in the job, and figure out which workloads are
// in question based on the running workloads and those defined in the
// repo. Fill in the release results along the way.
func (s ReleaseImageSpec) selectWorkloads(ctx context.Context, rc ReleaseContext, results Result) ([]*WorkloadUpdate, error) {
	// Build list of filters
	prefilters, postfilters, err := s.filters(rc)
	if err != nil {
		return nil, err
	}
	// Find and filter workloads
	return rc.SelectWorkloads(ctx, results, prefilters, postfilters)
}

func (s ReleaseImageSpec) filters(rc ReleaseContext) ([]WorkloadFilter, []WorkloadFilter, error) {
	var prefilters, postfilters []WorkloadFilter

	ids := []resource.ID{}
	for _, ss := range s.ServiceSpecs {
		if ss == ResourceSpecAll {
			// "<all>" Overrides any other filters
			ids = []resource.ID{}
			break
		}
		id, err := resource.ParseID(string(ss))
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

	// Filter out locked controllers unless given a specific controller(s) and forced
	if !(len(ids) > 0 && s.Force) {
		postfilters = append(postfilters, &LockedFilter{}, &IgnoreFilter{})
	}

	return prefilters, postfilters, nil
}

func (s ReleaseImageSpec) markSkipped(results Result) {
	for _, v := range s.ServiceSpecs {
		if v == ResourceSpecAll {
			continue
		}
		id, err := v.AsID()
		if err != nil {
			continue
		}
		if _, ok := results[id]; !ok {
			results[id] = WorkloadResult{
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
func (s ReleaseImageSpec) calculateImageUpdates(rc ReleaseContext, candidates []*WorkloadUpdate, results Result, logger log.Logger) ([]*WorkloadUpdate, error) {
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
			// FIXME(fons): we probably want to allow this operation even if image
			//              scanning is disabled. We could either avoid the validation
			//              or use an uncached registry.
			imageRepos, err = exactImageRepos(rc.Registry(), []image.Ref{ref})
		}
	}

	if err != nil {
		return nil, err
	}

	// Look through all the workloads' containers to see which have an
	// image that could be updated.
	var updates []*WorkloadUpdate
	for _, u := range candidates {
		containers, err := u.Workload.ContainersOrError()
		if err != nil {
			results[u.ResourceID] = WorkloadResult{
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

			tagPattern := policy.PatternAll
			// Use the container's filter if the spec does not want to force release, or
			// all images requested
			if !s.Force || s.ImageSpec == ImageSpecLatest {
				if pattern, ok := u.Resource.Policies().Get(policy.TagPrefix(container.Name)); ok {
					tagPattern = policy.NewPattern(pattern)
				}
			}

			metadata := imageRepos.GetRepositoryMetadata(currentImageID.Name)
			sortedImages, err := FilterAndSortRepositoryMetadata(metadata, tagPattern)
			if err != nil {
				// missing image repository metadata
				ignoredOrSkipped = ReleaseStatusUnknown
				continue
			}
			latestImage, ok := sortedImages.Latest()
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
			results[u.ResourceID] = WorkloadResult{
				Status:       ReleaseStatusSuccess,
				PerContainer: containerUpdates,
			}
		case ignoredOrSkipped == ReleaseStatusSkipped:
			results[u.ResourceID] = WorkloadResult{
				Status: ReleaseStatusSkipped,
				Error:  ImageUpToDate,
			}
		case ignoredOrSkipped == ReleaseStatusIgnored:
			results[u.ResourceID] = WorkloadResult{
				Status: ReleaseStatusIgnored,
				Error:  DoesNotUseImage,
			}
		case ignoredOrSkipped == ReleaseStatusUnknown:
			results[u.ResourceID] = WorkloadResult{
				Status: ReleaseStatusSkipped,
				Error:  ImageNotFound,
			}
		}
	}

	return updates, nil
}

type ResourceSpec string // ID or "<all>"

func ParseResourceSpec(s string) (ResourceSpec, error) {
	if s == string(ResourceSpecAll) {
		return ResourceSpecAll, nil
	}
	id, err := resource.ParseID(s)
	if err != nil {
		return "", errors.Wrap(err, "invalid workload spec")
	}
	return ResourceSpec(id.String()), nil
}

func MakeResourceSpec(id resource.ID) ResourceSpec {
	return ResourceSpec(id.String())
}

func (s ResourceSpec) AsID() (resource.ID, error) {
	return resource.ParseID(string(s))
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
