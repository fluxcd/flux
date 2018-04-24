package update

import (
	"fmt"
	"sort"
	"strings"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/image"
)

type ControllerUpdateStatus string

const (
	ReleaseStatusSuccess ControllerUpdateStatus = "success"
	ReleaseStatusFailed  ControllerUpdateStatus = "failed"
	ReleaseStatusSkipped ControllerUpdateStatus = "skipped"
	ReleaseStatusIgnored ControllerUpdateStatus = "ignored"
	ReleaseStatusUnknown ControllerUpdateStatus = "unknown"
)

type Result map[flux.ResourceID]ControllerResult

func (r Result) ServiceIDs() []string {
	var result []string
	for id := range r {
		result = append(result, id.String())
	}
	sort.StringSlice(result).Sort()
	return result
}

func (r Result) AffectedResources() flux.ResourceIDs {
	ids := flux.ResourceIDs{}
	for id, result := range r {
		if result.Status == ReleaseStatusSuccess {
			ids = append(ids, id)
		}
	}
	return ids
}

func (r Result) ChangedImages() []string {
	images := map[image.Ref]struct{}{}
	for _, serviceResult := range r {
		if serviceResult.Status != ReleaseStatusSuccess {
			continue
		}
		for _, containerResult := range serviceResult.PerContainer {
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
	for id, serviceResult := range r {
		if serviceResult.Status == ReleaseStatusFailed {
			errIds = append(errIds, id.String())
			errStr = serviceResult.Error
		}
	}
	switch {
	case len(errIds) == 0:
		return ""
	case len(errIds) == 1:
		return fmt.Sprintf("%s failed: %s", errIds[0], errStr)
	default:
		return fmt.Sprintf("Multiple services failed: %s", strings.Join(errIds, ", "))
	}
}

type ControllerResult struct {
	Status       ControllerUpdateStatus // summary of what happened, e.g., "incomplete", "ignored", "success"
	Error        string                 `json:",omitempty"` // error if there was one finding the service (e.g., it doesn't exist in repo)
	PerContainer []ContainerUpdate      // what happened with each container
}

type ContainerUpdate struct {
	Container string
	Current   image.Ref
	Target    image.Ref
}
