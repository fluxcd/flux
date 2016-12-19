package flux

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const (
	ServiceSpecAll  = ServiceSpec("<all>")
	ImageSpecLatest = ImageSpec("<all latest>")
	ImageSpecNone   = ImageSpec("<no updates>")
	PolicyNone      = Policy("")
	PolicyLocked    = Policy("locked")
	PolicyAutomated = Policy("automated")
)

var (
	ErrInvalidServiceID   = errors.New("invalid service ID")
	ErrInvalidImageID     = errors.New("invalid image ID")
	ErrInvalidReleaseKind = errors.New("invalid release kind")
)

type Token string

func (t Token) Set(req *http.Request) {
	if string(t) != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Scope-Probe token=%s", t))
	}
}

type InstanceID string

const InstanceIDHeaderKey = "X-Scope-OrgID"

const DefaultInstanceID = "<default-instance-id>"

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

type ServiceID string // "default/helloworld"

func ParseServiceID(s string) (ServiceID, error) {
	toks := strings.SplitN(s, "/", 2)
	if len(toks) != 2 {
		return "", ErrInvalidServiceID
	}
	return ServiceID(s), nil
}

func MakeServiceID(namespace, service string) ServiceID {
	return ServiceID(namespace + "/" + service)
}

func (id ServiceID) Components() (namespace, service string) {
	toks := strings.SplitN(string(id), "/", 2)
	if len(toks) != 2 {
		panic("invalid service spec")
	}
	return toks[0], toks[1]
}

type ServiceIDSet map[ServiceID]struct{}

func (s ServiceIDSet) Add(ids []ServiceID) {
	for _, id := range ids {
		s[id] = struct{}{}
	}
}

func (s ServiceIDSet) Without(others ServiceIDSet) ServiceIDSet {
	if s == nil || len(s) == 0 || others == nil || len(others) == 0 {
		return s
	}
	res := ServiceIDSet{}
	for id := range s {
		if !others.Contains(id) {
			res[id] = struct{}{}
		}
	}
	return res
}

func (s ServiceIDSet) Contains(id ServiceID) bool {
	if s == nil {
		return false
	}
	_, ok := s[id]
	return ok
}

func (s ServiceIDSet) Intersection(others ServiceIDSet) ServiceIDSet {
	if s == nil {
		return others
	}
	if others == nil {
		return s
	}
	result := ServiceIDSet{}
	for id := range s {
		if _, ok := others[id]; ok {
			result[id] = struct{}{}
		}
	}
	return result
}

type ServiceIDs []ServiceID

func (ids ServiceIDs) Without(others ServiceIDSet) (res ServiceIDs) {
	for _, id := range ids {
		if !others.Contains(id) {
			res = append(res, id)
		}
	}
	return res
}

func (ids ServiceIDs) Contains(id ServiceID) bool {
	set := ServiceIDSet{}
	set.Add(ids)
	return set.Contains(id)
}

func (ids ServiceIDs) Intersection(others ServiceIDSet) ServiceIDSet {
	set := ServiceIDSet{}
	set.Add(ids)
	return set.Intersection(others)
}

type ImageID string // "quay.io/weaveworks/helloworld:v1"

func ParseImageID(s string) ImageID {
	return ImageID(s) // technically all strings are valid
}

func MakeImageID(registry, name, tag string) ImageID {
	result := name
	if registry != "" {
		result = registry + "/" + name
	}
	if tag != "" {
		result = result + ":" + tag
	}
	return ImageID(result)
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
		return ServiceSpecAll, nil
	}
	id, err := ParseServiceID(s)
	if err != nil {
		return "", errors.Wrap(err, "invalid service spec")
	}
	return ServiceSpec(id), nil
}

func (s ServiceSpec) AsID() (ServiceID, error) {
	return ParseServiceID(string(s))
}

// ImageSpec is an ImageID, or "<all latest>" (update all containers
// to the latest available), or "<no updates>" (do not update any
// images)
type ImageSpec string

func ParseImageSpec(s string) ImageSpec {
	if s == string(ImageSpecLatest) {
		return ImageSpec(s)
	}
	return ImageSpec(ParseImageID(s))
}

type ImageStatus struct {
	ID         ServiceID
	Containers []Container
}

// Policy is an string, denoting the current deployment policy of a service,
// e.g. automated, or locked.
type Policy string

func ParsePolicy(s string) Policy {
	for _, p := range []Policy{
		PolicyLocked,
		PolicyAutomated,
	} {
		if s == string(p) {
			return p
		}
	}
	return PolicyNone
}

type ServiceStatus struct {
	ID         ServiceID
	Containers []Container
	Status     string
	Automated  bool
	Locked     bool
}

func (s ServiceStatus) Policies() string {
	var ps []string
	if s.Automated {
		ps = append(ps, string(PolicyAutomated))
	}
	if s.Locked {
		ps = append(ps, string(PolicyLocked))
	}
	sort.Strings(ps)
	return strings.Join(ps, ",")
}

type Container struct {
	Name      string
	Current   ImageDescription
	Available []ImageDescription
}

type ImageDescription struct {
	ID        ImageID
	CreatedAt *time.Time `json:",omitempty"`
}

// Ask me for more details.
type HistoryEntry struct {
	Stamp *time.Time `json:",omitempty"`
	Type  string
	Data  string
}

// TODO: This should also check we have access to any provided repositories.
// How similar should it be to the `get-config` result?
type Status struct {
	Fluxd FluxdStatus `json:"fluxd" yaml:"fluxd"`
	Git   GitStatus   `json:"git" yaml:"git"`
}

// TODO: This should include the fluxd version
type FluxdStatus struct {
	Connected bool `json:"connected" yaml:"connected"`
}

type GitStatus struct {
	Configured bool `json:"configured" yaml:"configured"`
}
