package flux

import (
	"sort"
	"time"

	"github.com/weaveworks/flux/guid"
)

const (
	PendingReleaseStatus = "pending"
	RunningReleaseStatus = "running"
	SuccessReleaseStatus = "success"
	FailedReleaseStatus  = "failed"
	SkippedReleaseStatus = "skipped"
)

type ServiceReleaseStatus string

type ReleaseID string

func NewReleaseID() ReleaseID {
	return ReleaseID(guid.New())
}

// Release describes a release
type Release struct {
	ID        ReleaseID            `json:"id"`
	CreatedAt time.Time            `json:"createdAt"`
	StartedAt time.Time            `json:"startedAt"`
	EndedAt   time.Time            `json:"endedAt"`
	Done      bool                 `json:"done"`
	Priority  int                  `json:"priority"`
	Status    ServiceReleaseStatus `json:"status"`
	Log       []string             `json:"log"`

	Spec   ReleaseSpec   `json:"spec"`
	Result ReleaseResult `json:"result"`
}

// NB: these get sent from fluxctl, so we have to maintain the json format of
// this. Eugh.
type ReleaseSpec struct {
	ServiceSpecs []ServiceSpec
	ImageSpec    ImageSpec
	Kind         ReleaseKind
	Excludes     []ServiceID

	// Backwards Compatibility, remove once no more jobs
	// TODO: Remove this once there are no more jobs with ServiceSpec, only ServiceSpecs
	ServiceSpec ServiceSpec
}

type ReleaseResult map[ServiceID]ServiceResult

func (r ReleaseResult) ServiceIDs() []string {
	var result []string
	for id := range r {
		result = append(result, string(id))
	}
	sort.StringSlice(result).Sort()
	return result
}

func (r ReleaseResult) ImageIDs() []string {
	images := map[ImageID]struct{}{}
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
func (r ReleaseResult) Error() string {
	for _, serviceResult := range r {
		if serviceResult.Error != "" {
			return serviceResult.Error
		}
	}
	return ""
}

type ServiceResult struct {
	Status       ServiceReleaseStatus // summary of what happened, e.g., "incomplete", "ignored", "success"
	Error        string               // error if there was one finding the service (e.g., it doesn't exist in repo)
	PerContainer []ContainerResult    // what happened with each container
}

type ContainerResult struct {
	ContainerUpdate
	Error string // error in upgrading, if one occured
}

type ContainerUpdate struct {
	Container string
	Current   ImageID
	Target    ImageID
}
