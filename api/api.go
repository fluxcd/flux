package api

import (
	"context"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/update"
)

// API for clients connecting to the service
type Server interface {
	Export(context.Context) ([]byte, error)
	ListServices(ctx context.Context, namespace string) ([]flux.ControllerStatus, error)
	ListImages(context.Context, update.ResourceSpec) ([]flux.ImageStatus, error)
	UpdateManifests(context.Context, update.Spec) (job.ID, error)
	SyncStatus(ctx context.Context, ref string) ([]string, error)
	JobStatus(context.Context, job.ID) (job.Status, error)
	GitRepoConfig(ctx context.Context, regenerate bool) (flux.GitConfig, error)
}
