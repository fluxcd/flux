package flux

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

var (
	ErrInvalidServiceID = errors.New("invalid service ID")

	LegacyServiceIDRegexp = regexp.MustCompile("^([a-zA-Z0-9_-]+)/([a-zA-Z0-9_-]+)$")
	// The namespace and name components are (apparently
	// non-normatively) defined in
	// https://github.com/kubernetes/community/blob/master/contributors/design-proposals/architecture/identifiers.md
	// In practice, more punctuation is used than allowed there;
	// specifically, people use underscores as well as dashes and dots, and in names, colons.
	ResourceIDRegexp            = regexp.MustCompile("^(<cluster>|[a-zA-Z0-9_-]+):([a-zA-Z0-9_-]+)/([a-zA-Z0-9_.:-]+)$")
	UnqualifiedResourceIDRegexp = regexp.MustCompile("^([a-zA-Z0-9_-]+)/([a-zA-Z0-9_.:-]+)$")
)

// ResourceID is an opaque type which uniquely identifies a resource in an
// orchestrator.
type ResourceID struct {
	resourceIDImpl
}

type resourceIDImpl interface {
	String() string
}

// Old-style <namespace>/<servicename> format
type legacyServiceID struct {
	namespace, service string
}

func (id legacyServiceID) String() string {
	return fmt.Sprintf("%s/%s", id.namespace, id.service)
}

// New <namespace>:<kind>/<name> format
type resourceID struct {
	namespace, kind, name string
}

func (id resourceID) String() string {
	return fmt.Sprintf("%s:%s/%s", id.namespace, id.kind, id.name)
}

// ParseResourceID constructs a ResourceID from a string representation
// if possible, returning an error value otherwise.
func ParseResourceID(s string) (ResourceID, error) {
	if m := ResourceIDRegexp.FindStringSubmatch(s); m != nil {
		return ResourceID{resourceID{m[1], strings.ToLower(m[2]), m[3]}}, nil
	}
	if m := LegacyServiceIDRegexp.FindStringSubmatch(s); m != nil {
		return ResourceID{legacyServiceID{m[1], m[2]}}, nil
	}
	return ResourceID{}, errors.Wrap(ErrInvalidServiceID, "parsing "+s)
}

// MustParseResourceID constructs a ResourceID from a string representation,
// panicing if the format is invalid.
func MustParseResourceID(s string) ResourceID {
	id, err := ParseResourceID(s)
	if err != nil {
		panic(err)
	}
	return id
}

// ParseResourceIDOptionalNamespace constructs a ResourceID from either a fully
// qualified string representation, or an unqualified kind/name representation
// and the supplied namespace.
func ParseResourceIDOptionalNamespace(namespace, s string) (ResourceID, error) {
	if m := ResourceIDRegexp.FindStringSubmatch(s); m != nil {
		return ResourceID{resourceID{m[1], strings.ToLower(m[2]), m[3]}}, nil
	}
	if m := UnqualifiedResourceIDRegexp.FindStringSubmatch(s); m != nil {
		return ResourceID{resourceID{namespace, strings.ToLower(m[1]), m[2]}}, nil
	}
	return ResourceID{}, errors.Wrap(ErrInvalidServiceID, "parsing "+s)
}

// MakeResourceID constructs a ResourceID from constituent components.
func MakeResourceID(namespace, kind, name string) ResourceID {
	return ResourceID{resourceID{namespace, strings.ToLower(kind), name}}
}

// Components returns the constituent components of a ResourceID
func (id ResourceID) Components() (namespace, kind, name string) {
	switch impl := id.resourceIDImpl.(type) {
	case resourceID:
		return impl.namespace, impl.kind, impl.name
	case legacyServiceID:
		return impl.namespace, "service", impl.service
	default:
		panic("wrong underlying type")
	}
}

// MarshalJSON encodes a ResourceID as a JSON string. This is
// done to maintain backwards compatibility with previous flux
// versions where the ResourceID is a plain string.
func (id ResourceID) MarshalJSON() ([]byte, error) {
	if id.resourceIDImpl == nil {
		// Sadly needed as it's possible to construct an empty ResourceID literal
		return json.Marshal("")
	}
	return json.Marshal(id.String())
}

// UnmarshalJSON decodes a ResourceID from a JSON string. This is
// done to maintain backwards compatibility with previous flux
// versions where the ResourceID is a plain string.
func (id *ResourceID) UnmarshalJSON(data []byte) (err error) {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	if string(str) == "" {
		// Sadly needed as it's possible to construct an empty ResourceID literal
		*id = ResourceID{}
		return nil
	}
	*id, err = ParseResourceID(string(str))
	return err
}

// MarshalText encodes a ResourceID as a flat string; this is
// required because ResourceIDs are sometimes used as map keys.
func (id ResourceID) MarshalText() (text []byte, err error) {
	return []byte(id.String()), nil
}

// MarshalText decodes a ResourceID from a flat string; this is
// required because ResourceIDs are sometimes used as map keys.
func (id *ResourceID) UnmarshalText(text []byte) error {
	result, err := ParseResourceID(string(text))
	if err != nil {
		return err
	}
	*id = result
	return nil
}

type ResourceIDSet map[ResourceID]struct{}

func (s ResourceIDSet) String() string {
	var ids []string
	for id := range s {
		ids = append(ids, id.String())
	}
	return "{" + strings.Join(ids, ", ") + "}"
}

func (s ResourceIDSet) Add(ids []ResourceID) {
	for _, id := range ids {
		s[id] = struct{}{}
	}
}

func (s ResourceIDSet) Without(others ResourceIDSet) ResourceIDSet {
	if s == nil || len(s) == 0 || others == nil || len(others) == 0 {
		return s
	}
	res := ResourceIDSet{}
	for id := range s {
		if !others.Contains(id) {
			res[id] = struct{}{}
		}
	}
	return res
}

func (s ResourceIDSet) Contains(id ResourceID) bool {
	if s == nil {
		return false
	}
	_, ok := s[id]
	return ok
}

func (s ResourceIDSet) Intersection(others ResourceIDSet) ResourceIDSet {
	if s == nil {
		return others
	}
	if others == nil {
		return s
	}
	result := ResourceIDSet{}
	for id := range s {
		if _, ok := others[id]; ok {
			result[id] = struct{}{}
		}
	}
	return result
}

func (s ResourceIDSet) ToSlice() ResourceIDs {
	i := 0
	keys := make(ResourceIDs, len(s))
	for k := range s {
		keys[i] = k
		i++
	}
	return keys
}

type ResourceIDs []ResourceID

func (p ResourceIDs) Len() int           { return len(p) }
func (p ResourceIDs) Less(i, j int) bool { return p[i].String() < p[j].String() }
func (p ResourceIDs) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p ResourceIDs) Sort()              { sort.Sort(p) }

func (ids ResourceIDs) Without(others ResourceIDSet) (res ResourceIDs) {
	for _, id := range ids {
		if !others.Contains(id) {
			res = append(res, id)
		}
	}
	return res
}

func (ids ResourceIDs) Contains(id ResourceID) bool {
	set := ResourceIDSet{}
	set.Add(ids)
	return set.Contains(id)
}

func (ids ResourceIDs) Intersection(others ResourceIDSet) ResourceIDSet {
	set := ResourceIDSet{}
	set.Add(ids)
	return set.Intersection(others)
}
