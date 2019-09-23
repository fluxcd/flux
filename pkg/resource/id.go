package resource

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
	IDRegexp            = regexp.MustCompile("^(<cluster>|[a-zA-Z0-9_-]+):([a-zA-Z0-9_-]+)/([a-zA-Z0-9_.:-]+)$")
	UnqualifiedIDRegexp = regexp.MustCompile("^([a-zA-Z0-9_-]+)/([a-zA-Z0-9_.:-]+)$")
)

// ID is an opaque type which uniquely identifies a resource in an
// orchestrator.
type ID struct {
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

// ParseID constructs a ID from a string representation
// if possible, returning an error value otherwise.
func ParseID(s string) (ID, error) {
	if m := IDRegexp.FindStringSubmatch(s); m != nil {
		return ID{resourceID{m[1], strings.ToLower(m[2]), m[3]}}, nil
	}
	if m := LegacyServiceIDRegexp.FindStringSubmatch(s); m != nil {
		return ID{legacyServiceID{m[1], m[2]}}, nil
	}
	return ID{}, errors.Wrap(ErrInvalidServiceID, "parsing "+s)
}

// MustParseID constructs a ID from a string representation,
// panicing if the format is invalid.
func MustParseID(s string) ID {
	id, err := ParseID(s)
	if err != nil {
		panic(err)
	}
	return id
}

// ParseIDOptionalNamespace constructs a ID from either a fully
// qualified string representation, or an unqualified kind/name representation
// and the supplied namespace.
func ParseIDOptionalNamespace(namespace, s string) (ID, error) {
	if m := IDRegexp.FindStringSubmatch(s); m != nil {
		return ID{resourceID{m[1], strings.ToLower(m[2]), m[3]}}, nil
	}
	if m := UnqualifiedIDRegexp.FindStringSubmatch(s); m != nil {
		return ID{resourceID{namespace, strings.ToLower(m[1]), m[2]}}, nil
	}
	return ID{}, errors.Wrap(ErrInvalidServiceID, "parsing "+s)
}

// MakeID constructs a ID from constituent components.
func MakeID(namespace, kind, name string) ID {
	return ID{resourceID{namespace, strings.ToLower(kind), name}}
}

// Components returns the constituent components of a ID
func (id ID) Components() (namespace, kind, name string) {
	switch impl := id.resourceIDImpl.(type) {
	case resourceID:
		return impl.namespace, impl.kind, impl.name
	case legacyServiceID:
		return impl.namespace, "service", impl.service
	default:
		panic("wrong underlying type")
	}
}

// MarshalJSON encodes a ID as a JSON string. This is
// done to maintain backwards compatibility with previous flux
// versions where the ID is a plain string.
func (id ID) MarshalJSON() ([]byte, error) {
	if id.resourceIDImpl == nil {
		// Sadly needed as it's possible to construct an empty ID literal
		return json.Marshal("")
	}
	return json.Marshal(id.String())
}

// UnmarshalJSON decodes a ID from a JSON string. This is
// done to maintain backwards compatibility with previous flux
// versions where the ID is a plain string.
func (id *ID) UnmarshalJSON(data []byte) (err error) {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	if string(str) == "" {
		// Sadly needed as it's possible to construct an empty ID literal
		*id = ID{}
		return nil
	}
	*id, err = ParseID(string(str))
	return err
}

// MarshalText encodes a ID as a flat string; this is
// required because ResourceIDs are sometimes used as map keys.
func (id ID) MarshalText() (text []byte, err error) {
	return []byte(id.String()), nil
}

// MarshalText decodes a ID from a flat string; this is
// required because ResourceIDs are sometimes used as map keys.
func (id *ID) UnmarshalText(text []byte) error {
	result, err := ParseID(string(text))
	if err != nil {
		return err
	}
	*id = result
	return nil
}

type IDSet map[ID]struct{}

func (s IDSet) String() string {
	var ids []string
	for id := range s {
		ids = append(ids, id.String())
	}
	return "{" + strings.Join(ids, ", ") + "}"
}

func (s IDSet) Add(ids []ID) {
	for _, id := range ids {
		s[id] = struct{}{}
	}
}

func (s IDSet) Without(others IDSet) IDSet {
	if s == nil || len(s) == 0 || others == nil || len(others) == 0 {
		return s
	}
	res := IDSet{}
	for id := range s {
		if !others.Contains(id) {
			res[id] = struct{}{}
		}
	}
	return res
}

func (s IDSet) Contains(id ID) bool {
	if s == nil {
		return false
	}
	_, ok := s[id]
	return ok
}

func (s IDSet) Intersection(others IDSet) IDSet {
	if s == nil {
		return others
	}
	if others == nil {
		return s
	}
	result := IDSet{}
	for id := range s {
		if _, ok := others[id]; ok {
			result[id] = struct{}{}
		}
	}
	return result
}

func (s IDSet) ToSlice() IDs {
	i := 0
	keys := make(IDs, len(s))
	for k := range s {
		keys[i] = k
		i++
	}
	return keys
}

type IDs []ID

func (p IDs) Len() int           { return len(p) }
func (p IDs) Less(i, j int) bool { return p[i].String() < p[j].String() }
func (p IDs) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p IDs) Sort()              { sort.Sort(p) }

func (ids IDs) Without(others IDSet) (res IDs) {
	for _, id := range ids {
		if !others.Contains(id) {
			res = append(res, id)
		}
	}
	return res
}

func (ids IDs) Contains(id ID) bool {
	set := IDSet{}
	set.Add(ids)
	return set.Contains(id)
}

func (ids IDs) Intersection(others IDSet) IDSet {
	set := IDSet{}
	set.Add(ids)
	return set.Intersection(others)
}
