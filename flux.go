package flux

import (
	"encoding/json"
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

type ServiceID struct {
	namespace string
	service   string
}

func (id ServiceID) String() string {
	return fmt.Sprintf("%s/%s", id.namespace, id.service)
}

func ParseServiceID(s string) (ServiceID, error) {
	toks := strings.SplitN(s, "/", 2)
	if len(toks) != 2 {
		return ServiceID{}, errors.Wrap(ErrInvalidServiceID, "parsing "+s)
	}
	return ServiceID{toks[0], toks[1]}, nil
}

func MustParseServiceID(s string) ServiceID {
	id, err := ParseServiceID(s)
	if err != nil {
		panic(err)
	}
	return id
}

func MakeServiceID(namespace, service string) ServiceID {
	return ServiceID{namespace, service}
}

func (id ServiceID) Components() (namespace, service string) {
	return id.namespace, id.service
}

func (id ServiceID) MarshalJSON() ([]byte, error) {
	return json.Marshal(id.String())
}

func (id *ServiceID) UnmarshalJSON(data []byte) (err error) {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	*id, err = ParseServiceID(string(str))
	return err
}

func (id ServiceID) MarshalText() (text []byte, err error) {
	return []byte(id.String()), nil
}

func (id *ServiceID) UnmarshalText(text []byte) error {
	result, err := ParseServiceID(string(text))
	if err != nil {
		return err
	}
	*id = result
	return nil
}

type ServiceIDSet map[ServiceID]struct{}

func (s ServiceIDSet) String() string {
	var ids []string
	for id := range s {
		ids = append(ids, id.String())
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
func (p ServiceIDs) Less(i, j int) bool { return p[i].String() < p[j].String() }
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
	Policies   map[string]string
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
