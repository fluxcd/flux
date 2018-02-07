package api

import (
	"context"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/update"
)

type ServerV4 interface {
	Ping(context.Context) error
	Version(context.Context) (string, error)
}

type ServerV5 interface {
	ServerV4

	Export(context.Context) ([]byte, error)
}

type ServerV6Deprecated interface {
	SyncNotify(context.Context) error
}

type ServerV6NotDeprecated interface {
	ServerV5

	ListServices(ctx context.Context, namespace string) ([]flux.ControllerStatus, error)
	ListImages(context.Context, update.ResourceSpec) ([]flux.ImageStatus, error)
	UpdateManifests(context.Context, update.Spec) (job.ID, error)
	SyncStatus(ctx context.Context, ref string) ([]string, error)
	JobStatus(context.Context, job.ID) (job.Status, error)
	GitRepoConfig(ctx context.Context, regenerate bool) (flux.GitConfig, error)
}

type ServerV6 interface {
	ServerV6Deprecated
	ServerV6NotDeprecated
}

type ServerV9 interface {
	ServerV6NotDeprecated

	// ChangeNotify tells the daemon that we've noticed a change in
	// e.g., the git repo, or image registry, and now would be a good
	// time to update its state.
	NotifyChange(context.Context, Change) error
}

// API for clients connecting to the service
type Server interface {
	ServerV9
}
