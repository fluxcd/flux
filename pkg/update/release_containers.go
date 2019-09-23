package update

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-kit/kit/log"

	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/resource"
)

var zeroImageRef = image.Ref{}

// ReleaseContainersSpec defines the spec for a `containers` manifest update.
type ReleaseContainersSpec struct {
	Kind           ReleaseKind
	ContainerSpecs map[resource.ID][]ContainerUpdate
	SkipMismatches bool
	Force          bool
}

// CalculateRelease computes required controller updates to satisfy this specification.
// It returns an error if any spec calculation fails unless `SkipMismatches` is true.
func (s ReleaseContainersSpec) CalculateRelease(ctx context.Context, rc ReleaseContext,
	logger log.Logger) ([]*WorkloadUpdate, Result, error) {
	results := Result{}
	prefilter, postfilter := s.filters()
	all, err := rc.SelectWorkloads(ctx, results, prefilter, postfilter)
	if err != nil {
		return nil, results, err
	}
	updates := s.workloadUpdates(results, all)
	return updates, results, s.resultsError(results)
}

func (s ReleaseContainersSpec) resultsError(results Result) error {
	failures := 0
	successes := 0
	for _, res := range results {
		switch res.Status {
		case ReleaseStatusFailed:
			failures++
		case ReleaseStatusSuccess:
			successes++
		}
	}
	if failures > 0 {
		return errors.New("cannot satisfy specs")
	}
	if successes == 0 {
		return errors.New("no changes found")
	}
	return nil
}

func (s ReleaseContainersSpec) filters() ([]WorkloadFilter, []WorkloadFilter) {
	var rids []resource.ID
	for rid := range s.ContainerSpecs {
		rids = append(rids, rid)
	}
	pre := []WorkloadFilter{&IncludeFilter{IDs: rids}}

	if !s.Force {
		return pre, []WorkloadFilter{&LockedFilter{}, &IgnoreFilter{}}
	}
	return pre, []WorkloadFilter{}
}

func (s ReleaseContainersSpec) workloadUpdates(results Result, all []*WorkloadUpdate) []*WorkloadUpdate {
	var updates []*WorkloadUpdate
	for _, u := range all {
		cs, err := u.Workload.ContainersOrError()
		if err != nil {
			results[u.ResourceID] = WorkloadResult{
				Status: ReleaseStatusFailed,
				Error:  err.Error(),
			}
			continue
		}

		containers := map[string]resource.Container{}
		for _, spec := range cs {
			containers[spec.Name] = spec
		}

		var mismatch, notfound []string
		var containerUpdates []ContainerUpdate
		for _, spec := range s.ContainerSpecs[u.ResourceID] {
			container, ok := containers[spec.Container]
			if !ok {
				notfound = append(notfound, spec.Container)
				continue
			}

			// An empty spec for the current image skips the precondition
			if spec.Current != zeroImageRef && container.Image != spec.Current {
				mismatch = append(mismatch, spec.Container)
				continue
			}

			if container.Image == spec.Target {
				// Nothing to update
				continue
			}

			containerUpdates = append(containerUpdates, spec)
		}

		mismatchError := fmt.Sprintf(ContainerTagMismatch, strings.Join(mismatch, ", "))

		var rerr string
		skippedMismatches := s.SkipMismatches && len(mismatch) > 0
		switch {
		case len(notfound) > 0:
			// Always fail if container disappeared or was misspelled
			results[u.ResourceID] = WorkloadResult{
				Status: ReleaseStatusFailed,
				Error:  fmt.Sprintf(ContainerNotFound, strings.Join(notfound, ", ")),
			}
		case !s.SkipMismatches && len(mismatch) > 0:
			// Only fail if we do not skip for mismatches. Otherwise we either succeed
			// with partial updates or then mark it as skipped because no precondition
			// fulfilled.
			results[u.ResourceID] = WorkloadResult{
				Status: ReleaseStatusFailed,
				Error:  mismatchError,
			}
		case len(containerUpdates) == 0:
			rerr = ImageUpToDate
			if skippedMismatches {
				rerr = mismatchError
			}
			results[u.ResourceID] = WorkloadResult{
				Status: ReleaseStatusSkipped,
				Error:  rerr,
			}
		default:
			rerr = ""
			if skippedMismatches {
				// While we succeed here, we still want the client to know that some
				// container mismatched.
				rerr = mismatchError
			}
			u.Updates = containerUpdates
			updates = append(updates, u)
			results[u.ResourceID] = WorkloadResult{
				Status:       ReleaseStatusSuccess,
				Error:        rerr,
				PerContainer: u.Updates,
			}
		}
	}

	return updates
}

func (s ReleaseContainersSpec) ReleaseKind() ReleaseKind {
	return s.Kind
}

func (s ReleaseContainersSpec) ReleaseType() ReleaseType {
	return "containers"
}

func (s ReleaseContainersSpec) CommitMessage(result Result) string {
	var workloads []string
	body := &bytes.Buffer{}
	for _, res := range result.AffectedResources() {
		workloads = append(workloads, res.String())
		fmt.Fprintf(body, "\n%s", res)
		for _, upd := range result[res].PerContainer {
			fmt.Fprintf(body, "\n- %s", upd.Target)
		}
		fmt.Fprintln(body)
	}
	if err := result.Error(); err != "" {
		fmt.Fprintf(body, "\n%s", result.Error())
	}
	return fmt.Sprintf("Update image refs in %s\n%s", strings.Join(workloads, ", "), body.String())
}
