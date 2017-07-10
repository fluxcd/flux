package policy

import (
	"encoding/json"
	"strings"

	"github.com/weaveworks/flux"
)

const (
	None      = Policy("")
	Ignore    = Policy("ignore")
	Locked    = Policy("locked")
	Automated = Policy("automated")
)

// Policy is an string, denoting the current deployment policy of a service,
// e.g. automated, or locked.
type Policy string

func Boolean(policy Policy) bool {
	switch policy {
	case Locked, Automated, Ignore:
		return true
	}
	return false
}

func TagPrefix(container string) Policy {
	return Policy("tag." + container)
}

func Tag(policy Policy) bool {
	return strings.HasPrefix(string(policy), "tag.")
}

type Updates map[flux.ServiceID]Update

type Update struct {
	Add    Set `json:"add"`
	Remove Set `json:"remove"`
}

type Set map[Policy]string

// We used to specify a set of policies as []Policy, and in some places
// it may be so serialised.
func (s *Set) UnmarshalJSON(in []byte) error {
	type set Set
	if err := json.Unmarshal(in, (*set)(s)); err != nil {
		var list []Policy
		if err = json.Unmarshal(in, &list); err != nil {
			return err
		}
		var s1 = Set{}
		*s = s1.Add(list...)
	}
	return nil
}

func (s Set) String() string {
	var ps []string
	for p, v := range s {
		ps = append(ps, string(p)+":"+v)
	}
	return "{" + strings.Join(ps, ", ") + "}"
}

func (s Set) Add(ps ...Policy) Set {
	s = clone(s)
	for _, p := range ps {
		s[p] = "true"
	}
	return s
}

func (s Set) Set(p Policy, v string) Set {
	s = clone(s)
	s[p] = v
	return s
}

func clone(s Set) Set {
	newMap := Set{}
	for p, v := range s {
		newMap[p] = v
	}
	return newMap
}

func (s Set) Contains(needle Policy) bool {
	for p, _ := range s {
		if p == needle {
			return true
		}
	}
	return false
}

func (s Set) Get(p Policy) (string, bool) {
	v, ok := s[p]
	return v, ok
}

type ServiceMap map[flux.ServiceID]Set

func (s ServiceMap) ToSlice() []flux.ServiceID {
	slice := []flux.ServiceID{}
	for service, _ := range s {
		slice = append(slice, service)
	}
	return slice
}

func (s ServiceMap) Contains(id flux.ServiceID) bool {
	_, ok := s[id]
	return ok
}

func (s ServiceMap) Without(other ServiceMap) ServiceMap {
	newMap := ServiceMap{}
	for k, v := range s {
		if !other.Contains(k) {
			newMap[k] = v
		}
	}
	return newMap
}
