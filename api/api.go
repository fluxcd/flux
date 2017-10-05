package api

import (
	"context"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/history"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/ssh"
	"github.com/weaveworks/flux/update"
)

// API for clients connecting to the service
type Client interface {
	ListServices(ctx context.Context, namespace string) ([]flux.ControllerStatus, error)
	ListImages(context.Context, update.ResourceSpec) ([]flux.ImageStatus, error)
	UpdateImages(context.Context, update.ReleaseSpec, update.Cause) (job.ID, error)
	SyncNotify(context.Context) error
	JobStatus(context.Context, job.ID) (job.Status, error)
	SyncStatus(ctx context.Context, ref string) ([]string, error)
	UpdatePolicies(context.Context, policy.Updates, update.Cause) (job.ID, error)
	Export(context.Context) ([]byte, error)
	PublicSSHKey(ctx context.Context, regenerate bool) (ssh.PublicKey, error)
}

// API for daemons connecting to an upstream service
type Upstream interface {
	RegisterDaemon(context.Context, remote.Platform) error
	IsDaemonConnected(context.Context) error
	LogEvent(context.Context, history.Event) error
}
