package flux

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
)

var (
	ErrInvalidServiceID = errors.New("invalid service ID")
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

type ServiceID string // "default/helloworld"

func (id ServiceID) String() string {
	return string(id)
}

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

func (s ServiceIDSet) String() string {
	var ids []string
	for id, _ := range s {
		ids = append(ids, string(id))
	}
	return "{" + strings.Join(ids, ", ") + "}"
}

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

func (s ServiceIDSet) ToSlice() ServiceIDs {
	i := 0
	keys := make(ServiceIDs, len(s))
	for k := range s {
		keys[i] = k
		i++
	}
	return keys
}

type ServiceIDs []ServiceID

func (p ServiceIDs) Len() int           { return len(p) }
func (p ServiceIDs) Less(i, j int) bool { return string(p[i]) < string(p[j]) }
func (p ServiceIDs) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p ServiceIDs) Sort()              { sort.Sort(p) }

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

type ImageStatus struct {
	ID         ServiceID
	Containers []Container
}

type ServiceStatus struct {
	ID         ServiceID
	Containers []Container
	Status     string
	Automated  bool
	Locked     bool
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

// TODO: How similar should this be to the `get-config` result?
type Status struct {
	Fluxsvc FluxsvcStatus `json:"fluxsvc" yaml:"fluxsvc"`
	Fluxd   FluxdStatus   `json:"fluxd" yaml:"fluxd"`
	Git     GitStatus     `json:"git" yaml:"git"`
}

type FluxsvcStatus struct {
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
}

type FluxdStatus struct {
	Connected bool   `json:"connected" yaml:"connected"`
	Version   string `json:"version,omitempty" yaml:"version,omitempty"`
}

type GitStatus struct {
	Configured bool   `json:"configured" yaml:"configured"`
	Error      string `json:"error,omitempty" yaml:"error,omitempty"`
}
