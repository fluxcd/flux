package v6

import (
	"context"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/ssh"
	"github.com/weaveworks/flux/update"
)

type ImageStatus struct {
	ID         flux.ResourceID
	Containers []Container
}

type ControllerStatus struct {
	ID         flux.ResourceID
	Containers []Container
	Status     string
	Automated  bool
	Locked     bool
	Ignore     bool
	Policies   map[string]string
}

type Container struct {
	Name           string
	Current        image.Info
	Available      []image.Info
	AvailableError string `json:",omitempty"`
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
	ListImages(context.Context, update.ResourceSpec) ([]ImageStatus, error)
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
