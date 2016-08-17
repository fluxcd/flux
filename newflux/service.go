package flux

import (
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
)

type Service interface {
	ListServices() ([]ServiceDescription, error)
	ListImages(ServiceSpec) ([]ImageDescription, error)
	Release(ServiceSpec, ImageSpec) error
	Automate(ServiceID) error
	Deautomate(ServiceID) error
	History(ServiceSpec) ([]HistoryEntry, error)
}

const (
	ServiceSpecAll  = ServiceSpec("<all>")
	ImageSpecLatest = ImageSpec("<latest>")
)

var (
	ErrInvalidServiceID = errors.New("invalid service ID")
	ErrInvalidImageID   = errors.New("invalid image ID")
)

type ServiceID string // "default/helloworld"

func ParseServiceID(s string) (ServiceID, error) {
	toks := strings.SplitN(s, "/", 2)
	if len(toks) != 2 {
		return "", ErrInvalidServiceID
	}
	return ServiceID(s), nil
}

type ImageID string // "quay.io/weaveworks/helloworld:v1"

func ParseImageID(s string) ImageID {
	return ImageID(s) // technically all strings are valid
}

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

type ServiceSpec string // ServiceID or "<all>"

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

type ImageSpec string // ImageID or "<latest>"

func ParseImageSpec(s string) ImageSpec {
	if s == string(ImageSpecLatest) {
		return ImageSpec(s)
	}
	return ImageSpec(ParseImageID(s))
}

type ServiceDescription struct {
	ID         ServiceID
	Containers []Container
}

type Container struct {
	Name      string
	Current   ImageDescription
	Available []ImageDescription
}

type ImageDescription struct {
	ID        ImageID
	CreatedAt time.Time
}

// Ask me for more details.
type HistoryEntry struct {
	Stamp time.Time
	Type  string
	Data  string
}
