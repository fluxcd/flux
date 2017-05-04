package update

import (
	"fmt"
	"sort"

	"github.com/weaveworks/flux"
)

type ServiceUpdateStatus string

const (
	ReleaseStatusPending ServiceUpdateStatus = "pending"
	ReleaseStatusSuccess ServiceUpdateStatus = "success"
	ReleaseStatusFailed  ServiceUpdateStatus = "failed"
	ReleaseStatusSkipped ServiceUpdateStatus = "skipped"
	ReleaseStatusIgnored ServiceUpdateStatus = "ignored"
	ReleaseStatusUnknown ServiceUpdateStatus = "unknown"
)

type Result map[flux.ServiceID]ServiceResult

func (r Result) ServiceIDs() []string {
	var result []string
	for id := range r {
		result = append(result, string(id))
	}
	sort.StringSlice(result).Sort()
	return result
}

func (r Result) ImageIDs() []string {
	images := map[flux.ImageID]struct{}{}
	for _, serviceResult := range r {
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
// TODO: should we concat them here? or what if there are multiple?
func (r Result) Error() string {
	for _, serviceResult := range r {
		if serviceResult.Error != "" {
			return serviceResult.Error
		}
	}
	return ""
}

type ServiceResult struct {
	Status       ServiceUpdateStatus // summary of what happened, e.g., "incomplete", "ignored", "success"
	Error        string              `json:",omitempty"` // error if there was one finding the service (e.g., it doesn't exist in repo)
	PerContainer []ContainerUpdate   // what happened with each container
}

func (fr ServiceResult) Msg(id flux.ServiceID) string {
	return fmt.Sprintf("%s service %s as it is %s", fr.Status, id.String(), fr.Error)
}

type ContainerUpdate struct {
	Container string
	Current   flux.ImageID
	Target    flux.ImageID
}
