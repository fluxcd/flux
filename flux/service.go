package flux

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

// These package-level errors should be self-explanatory.
var (
	ErrInvalidServiceID   = errors.New("invalid service ID")
	ErrInvalidImageID     = errors.New("invalid image ID")
	ErrInvalidReleaseKind = errors.New("invalid release kind")
)

// These spec constants should be self-explanatory.
const (
	ServiceSpecAll  = ServiceSpec("<all>")
	ImageSpecLatest = ImageSpec("<all latest>")
	ImageSpecNone   = ImageSpec("<no updates>")
)

// ServiceID identifies a specific service.
type ServiceID string // "default/helloworld"

// ParseServiceID parses a ServiceID from a string.
func ParseServiceID(s string) (ServiceID, error) {
	toks := strings.SplitN(s, "/", 2)
	if len(toks) != 2 {
		return "", ErrInvalidServiceID
	}
	return ServiceID(s), nil
}

// MakeServiceID constructs a service ID from its namespace and service name
// components.
func MakeServiceID(namespace, service string) ServiceID {
	return ServiceID(namespace + "/" + service)
}

// Components splits a ServiceID to its composite namespace and service name.
func (id ServiceID) Components() (namespace, service string) {
	toks := strings.SplitN(string(id), "/", 2)
	if len(toks) != 2 {
		panic("invalid service spec")
	}
	return toks[0], toks[1]
}

// ImageID identifies a specific service, as qualified as it possibly can be.
type ImageID string // "quay.io/weaveworks/helloworld:v1"

// ParseImageID parses an image ID from a string.
// (Technically, all strings are valid image IDs.)
func ParseImageID(s string) ImageID {
	return ImageID(s)
}

// Components splits an image ID to its composite components.
// If something isn't specified, its component will be an empty string.
func (id ImageID) Components() (registry, name, tag string) {
	s := string(id)
	toks := strings.SplitN(s, "/", 3)
	if len(toks) == 3 {
		registry = toks[0]
		s = fmt.Sprintf("%s/%s", toks[1], toks[2])
	}
	toks = strings.SplitN(s, ":", 2)
	if len(toks) == 2 {
		tag = toks[1]
	}
	name = toks[0]
	return registry, name, tag
}

// Repository returns the repository component of the image ID.
// There is some logic to do this intelligently.
func (id ImageID) Repository() string {
	registry, name, _ := id.Components()
	if registry != "" && name != "" {
		return registry + "/" + name
	}
	if name != "" {
		return name
	}
	return ""
}

// ServiceSpec is an ImageID, or "<all>" (all services on the platform).
type ServiceSpec string // ServiceID or "<all>"

// ParseServiceSpec parses a ServiceSpec from a string.
func ParseServiceSpec(s string) (ServiceSpec, error) {
	if s == string(ServiceSpecAll) {
		return ServiceSpec(s), nil
	}
	id, err := ParseServiceID(s)
	if err != nil {
		return "", errors.Wrap(err, "invalid service spec")
	}
	return ServiceSpec(id), nil
}

// ImageSpec is an ImageID, or "<all latest>" (update all containers
// to the latest available), or "<no updates>" (do not update any
// images).
type ImageSpec string

// ParseImageSpec parses an ImageSpec from a string.
func ParseImageSpec(s string) ImageSpec {
	if s == string(ImageSpecLatest) {
		return ImageSpec(s)
	}
	return ImageSpec(ParseImageID(s))
}

// ReleaseKind indicates a dry-run or normal release.
type ReleaseKind string

// I hope these enumerations are self-explanatory.
const (
	ReleaseKindPlan    ReleaseKind = "plan"
	ReleaseKindExecute             = "execute"
)

// ParseReleaseKind converts a string to a release kind.
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
