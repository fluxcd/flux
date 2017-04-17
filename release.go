package flux

import (
	"sort"
	"time"

	"fmt"
	"github.com/weaveworks/flux/guid"
)

// ReleaseKind says whether a release is to be planned only, or planned then executed
type ReleaseKind string

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

const (
	ReleaseStatusPending ServiceReleaseStatus = "pending"
	ReleaseStatusSuccess ServiceReleaseStatus = "success"
	ReleaseStatusFailed  ServiceReleaseStatus = "failed"
	ReleaseStatusSkipped ServiceReleaseStatus = "skipped"
	ReleaseStatusIgnored ServiceReleaseStatus = "ignored"
	ReleaseStatusUnknown ServiceReleaseStatus = "unknown"
)

type ServiceReleaseStatus string

type ReleaseID string

func NewReleaseID() ReleaseID {
	return ReleaseID(guid.New())
}

// How did this release get triggered?
type ReleaseCause struct {
	Message string
	User    string
}

const UserAutomated = "<automated>"

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

	Cause  ReleaseCause  `json:"cause"`
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
}

// ReleaseType gives a one-word description of the release, mainly
// useful for labelling metrics or log messages.
func (s ReleaseSpec) ReleaseType() string {
	switch {
	case s.ImageSpec == ImageSpecLatest:
		return "latest_images"
	case s.ImageSpec == ImageSpecNone:
		return "config_only"
	default:
		return "specific_image"
	}
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
	Error        string               `json:",omitempty"` // error if there was one finding the service (e.g., it doesn't exist in repo)
	PerContainer []ContainerUpdate    // what happened with each container
}

func (fr ServiceResult) Msg(id ServiceID) string {
	return fmt.Sprintf("%s service %s as it is %s", fr.Status, id.String(), fr.Error)
}

type ContainerUpdate struct {
	Container string
	Current   ImageID
	Target    ImageID
}
