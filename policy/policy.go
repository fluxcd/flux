package policy

import (
	"strings"

	"github.com/weaveworks/flux"
)

const (
	None      = Policy("")
	Locked    = Policy("locked")
	Automated = Policy("automated")
)

// Policy is an string, denoting the current deployment policy of a service,
// e.g. automated, or locked.
type Policy string

func Parse(s string) Policy {
	for _, p := range []Policy{
		Locked,
		Automated,
	} {
		if s == string(p) {
			return p
		}
	}
	return None
}

type Updates map[flux.ServiceID]Update

type Update struct {
	Add    []Policy `json:"add"`
	Remove []Policy `json:"remove"`
}

type PolicySet []Policy

func (s PolicySet) String() string {
	var ps []string
	for _, p := range s {
		ps = append(ps, string(p))
	}
	return "{" + strings.Join(ps, ", ") + "}"
}

func (s PolicySet) Add(ps ...Policy) PolicySet {
	dedupe := map[Policy]struct{}{}
	for _, p := range s {
		dedupe[p] = struct{}{}
	}
	for _, p := range ps {
		dedupe[p] = struct{}{}
	}
	var result PolicySet
	for p := range dedupe {
		result = append(result, p)
	}
	return result
}

func (s PolicySet) Contains(needle Policy) bool {
	for _, p := range s {
		if p == needle {
			return true
		}
	}
	return false
}
