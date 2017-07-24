package flux

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/pkg/errors"

	"github.com/weaveworks/flux/ssh"
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

// (User) Service identifiers

type ServiceID string // "default/helloworld"

func (id ServiceID) String() string {
	return string(id)
}

func ParseServiceID(s string) (ServiceID, error) {
	toks := strings.SplitN(s, "/", 2)
	if len(toks) != 2 {
		return "", errors.Wrap(ErrInvalidServiceID, "parsing "+s)
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
	for id := range s {
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

// -- types used in API

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
	Ignore     bool
}

type Container struct {
	Name      string
	Current   Image
	Available []Image
}

// --- config types

func NewGitRemoteConfig(url, branch, path string) (GitRemoteConfig, error) {
	if len(path) > 0 && path[0] == '/' {
		return GitRemoteConfig{}, errors.New("git subdirectory (--git-path) should not have leading forward slash")
	}
	return GitRemoteConfig{
		URL:    url,
		Branch: branch,
		Path:   path,
	}, nil
}

type GitRemoteConfig struct {
	URL    string `json:"url"`
	Branch string `json:"branch"`
	Path   string `json:"path"`
}

type GitConfig struct {
	Remote       GitRemoteConfig `json:"remote"`
	PublicSSHKey ssh.PublicKey   `json:"publicSSHKey"`
}
