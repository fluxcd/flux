package update

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/resource"
)

type WorkloadUpdateStatus string

const (
	ReleaseStatusSuccess WorkloadUpdateStatus = "success"
	ReleaseStatusFailed  WorkloadUpdateStatus = "failed"
	ReleaseStatusSkipped WorkloadUpdateStatus = "skipped"
	ReleaseStatusIgnored WorkloadUpdateStatus = "ignored"
	ReleaseStatusUnknown WorkloadUpdateStatus = "unknown"
)

type Result map[resource.ID]WorkloadResult

func (r Result) WorkloadIDs() []string {
	var result []string
	for id := range r {
		result = append(result, id.String())
	}
	sort.StringSlice(result).Sort()
	return result
}

func (r Result) AffectedResources() resource.IDs {
	ids := resource.IDs{}
	for id, result := range r {
		if result.Status == ReleaseStatusSuccess {
			ids = append(ids, id)
		}
	}
	return ids
}

func (r Result) ChangedImages() []string {
	images := map[image.Ref]struct{}{}
	for _, workloadResult := range r {
		if workloadResult.Status != ReleaseStatusSuccess {
			continue
		}
		for _, containerResult := range workloadResult.PerContainer {
			images[containerResult.Target] = struct{}{}
		}
	}
	var result []string
	for image := range images {
		result = append(result, image.String())
	}
	sort.StringSlice(result).Sort()
	return result
}

// Error returns the error for this release (if any)
func (r Result) Error() string {
	var errIds []string
	var errStr string
	for id, workloadResult := range r {
		if workloadResult.Status == ReleaseStatusFailed {
			errIds = append(errIds, id.String())
			errStr = workloadResult.Error
		}
	}
	switch {
	case len(errIds) == 0:
		return ""
	case len(errIds) == 1:
		return fmt.Sprintf("%s failed: %s", errIds[0], errStr)
	default:
		return fmt.Sprintf("Multiple workloads failed: %s", strings.Join(errIds, ", "))
	}
}

type WorkloadResult struct {
	Status       WorkloadUpdateStatus // summary of what happened, e.g., "incomplete", "ignored", "success"
	Error        string               `json:",omitempty"` // error if there was one finding the service (e.g., it doesn't exist in repo)
	PerContainer []ContainerUpdate    // what happened with each container
}

type ContainerUpdate struct {
	Container string
	Current   image.Ref
	Target    image.Ref
}
