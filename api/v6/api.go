package v6

import (
	"context"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/ssh"
	"github.com/weaveworks/flux/update"
)

type ImageStatus struct {
	ID         flux.ResourceID
	Containers []Container
}

// ReadOnlyReason enumerates the reasons that a controller is
// considered read-only. The zero value is considered "OK", since the
// zero value is what prior versions of the daemon will effectively
// send.
type ReadOnlyReason string

// ReadOnlyOK signifies that the repo can be written to
const ReadOnlyOK ReadOnlyReason = ""

// ReadOnlyMissing it's in a cluster but not in the git repo
const ReadOnlyMissing ReadOnlyReason = "NotInRepo"

// ReadOnlySystem indicates that the repo is in control of kubernetes not by the repo
const ReadOnlySystem ReadOnlyReason = "System"

// ReadOnlyNoRepo indicates that the user has elected to not supply a git repo
const ReadOnlyNoRepo ReadOnlyReason = "NoRepo"

// ReadOnlyNotReady indicates that Flux hasn't booted up yet
const ReadOnlyNotReady ReadOnlyReason = "NotReady"

const (
	// TestGoDoc is a test for Go Doc
	TestGoDoc = "asdf"

	SecondGoDocTest = "qwerty" // SecondGoDocTest tests an inline comment
)

type ControllerStatus struct {
	ID         flux.ResourceID
	Containers []Container
	ReadOnly   ReadOnlyReason
	Status     string
	Rollout    cluster.RolloutStatus
	SyncError  string
	Antecedent flux.ResourceID
	Labels     map[string]string
	Automated  bool
	Locked     bool
	Ignore     bool
	Policies   map[string]string
}

// --- config types

type GitRemoteConfig struct {
	URL    string `json:"url"`
	Branch string `json:"branch"`
	Path   string `json:"path"`
}

type GitConfig struct {
	Remote       GitRemoteConfig   `json:"remote"`
	PublicSSHKey ssh.PublicKey     `json:"publicSSHKey"`
	Status       git.GitRepoStatus `json:"status"`
}

type Deprecated interface {
	SyncNotify(context.Context) error
}

type NotDeprecated interface {
	// from v5
	Export(context.Context) ([]byte, error)

	// v6
	ListServices(ctx context.Context, namespace string) ([]ControllerStatus, error)
	ListImages(ctx context.Context, spec update.ResourceSpec) ([]ImageStatus, error)
	UpdateManifests(context.Context, update.Spec) (job.ID, error)
	SyncStatus(ctx context.Context, ref string) ([]string, error)
	JobStatus(context.Context, job.ID) (job.Status, error)
	GitRepoConfig(ctx context.Context, regenerate bool) (GitConfig, error)
}

type Upstream interface {
	// from v4
	Ping(context.Context) error
	Version(context.Context) (string, error)
}

type Server interface {
	Deprecated
	NotDeprecated
}
