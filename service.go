package flux

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
)

type Service interface {
	ListServices(inst InstanceID, namespace string) ([]ServiceStatus, error)
	ListImages(InstanceID, ServiceSpec) ([]ImageStatus, error)
	PostRelease(InstanceID, ReleaseJobSpec) (ReleaseID, error)
	GetRelease(InstanceID, ReleaseID) (ReleaseJob, error)
	Automate(InstanceID, ServiceID) error
	Deautomate(InstanceID, ServiceID) error
	Lock(InstanceID, ServiceID) error
	Unlock(InstanceID, ServiceID) error
	History(InstanceID, ServiceSpec) ([]HistoryEntry, error)
}

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
	CreatedAt time.Time
}

// Ask me for more details.
type HistoryEntry struct {
	Stamp time.Time
	Type  string
	Data  string
}

// ---

type ReleaseJobStore interface {
	ReleaseJobReadPusher
	ReleaseJobWritePopper
	GC() error
}

type ReleaseJobReadPusher interface {
	GetJob(InstanceID, ReleaseID) (ReleaseJob, error)
	PutJob(InstanceID, ReleaseJobSpec) (ReleaseID, error)
}

type ReleaseJobWritePopper interface {
	ReleaseJobUpdater
	ReleaseJobPopper
}

type ReleaseJobUpdater interface {
	UpdateJob(ReleaseJob) error
}

type ReleaseJobPopper interface {
	NextJob() (ReleaseJob, error)
}

var (
	ErrNoSuchReleaseJob      = errors.New("no such release job found")
	ErrNoReleaseJobAvailable = errors.New("no release job available")
)

type ReleaseID string

func NewReleaseID() ReleaseID {
	b := make([]byte, 16)
	rand.Read(b)
	return ReleaseID(fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]))
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

type ReleaseJob struct {
	Instance  InstanceID     `json:"instanceID"`
	Spec      ReleaseJobSpec `json:"spec"`
	ID        ReleaseID      `json:"id"`
	Submitted time.Time      `json:"submitted"`
	Claimed   time.Time      `json:"claimed,omitempty"`
	Started   time.Time      `json:"started,omitempty"`
	Status    string         `json:"status"`
	Log       []string       `json:"log,omitempty"`
	Finished  time.Time      `json:"finished,omitempty"`
	Success   bool           `json:"success"` // only makes sense after Finished
}

func (job *ReleaseJob) IsFinished() bool {
	return !job.Finished.IsZero()
}

type ReleaseJobSpec struct {
	ServiceSpec ServiceSpec
	ImageSpec   ImageSpec
	Kind        ReleaseKind
	Excludes    []ServiceID
}
