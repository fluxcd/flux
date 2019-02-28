package remote

import (
	"context"

	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/api/v10"
	"github.com/weaveworks/flux/api/v11"
	"github.com/weaveworks/flux/api/v6"
	"github.com/weaveworks/flux/api/v9"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/update"
)

var _ api.Server = &ErrorLoggingServer{}
var _ api.UpstreamServer = &ErrorLoggingUpstreamServer{}

type ErrorLoggingServer struct {
	server api.Server
	logger log.Logger
}

func NewErrorLoggingServer(s api.Server, l log.Logger) *ErrorLoggingServer {
	return &ErrorLoggingServer{s, l}
}

func (p *ErrorLoggingServer) Export(ctx context.Context) (config []byte, err error) {
	defer func() {
		if err != nil {
			// Omit config as it could be large
			p.logger.Log("method", "Export", "error", err)
		}
	}()
	return p.server.Export(ctx)
}

func (p *ErrorLoggingServer) ListWorkloads(ctx context.Context, maybeNamespace string) (_ []v6.WorkloadStatus, err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "ListWorkloads", "error", err)
		}
	}()
	return p.server.ListWorkloads(ctx, maybeNamespace)
}

func (p *ErrorLoggingServer) ListWorkloadsWithOptions(ctx context.Context, opts v11.ListWorkloadsOptions) (_ []v6.WorkloadStatus, err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "ListWorkloadsWithOptions", "error", err)
		}
	}()
	return p.server.ListWorkloadsWithOptions(ctx, opts)
}

func (p *ErrorLoggingServer) ListImages(ctx context.Context, spec update.ResourceSpec) (_ []v6.ImageStatus, err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "ListImages", "error", err)
		}
	}()
	return p.server.ListImages(ctx, spec)
}

func (p *ErrorLoggingServer) ListImagesWithOptions(ctx context.Context, opts v10.ListImagesOptions) (_ []v6.ImageStatus, err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "ListImagesWithOptions", "error", err)
		}
	}()
	return p.server.ListImagesWithOptions(ctx, opts)
}

func (p *ErrorLoggingServer) JobStatus(ctx context.Context, jobID job.ID) (_ job.Status, err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "JobStatus", "error", err)
		}
	}()
	return p.server.JobStatus(ctx, jobID)
}

func (p *ErrorLoggingServer) SyncStatus(ctx context.Context, ref string) (_ []string, err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "SyncStatus", "error", err)
		}
	}()
	return p.server.SyncStatus(ctx, ref)
}

func (p *ErrorLoggingServer) UpdateManifests(ctx context.Context, u update.Spec) (_ job.ID, err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "UpdateManifests", "error", err)
		}
	}()
	return p.server.UpdateManifests(ctx, u)
}

func (p *ErrorLoggingServer) GitRepoConfig(ctx context.Context, regenerate bool) (_ v6.GitConfig, err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "GitRepoConfig", "error", err)
		}
	}()
	return p.server.GitRepoConfig(ctx, regenerate)
}

type ErrorLoggingUpstreamServer struct {
	*ErrorLoggingServer
	server api.UpstreamServer
}

func NewErrorLoggingUpstreamServer(s api.UpstreamServer, l log.Logger) *ErrorLoggingUpstreamServer {
	return &ErrorLoggingUpstreamServer{
		NewErrorLoggingServer(s, l),
		s,
	}
}

func (p *ErrorLoggingUpstreamServer) Ping(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "Ping", "error", err)
		}
	}()
	return p.server.Ping(ctx)
}

func (p *ErrorLoggingUpstreamServer) Version(ctx context.Context) (v string, err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "Version", "error", err, "version", v)
		}
	}()
	return p.server.Version(ctx)
}

func (p *ErrorLoggingUpstreamServer) NotifyChange(ctx context.Context, change v9.Change) (err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "NotifyChange", "error", err)
		}
	}()
	return p.server.NotifyChange(ctx, change)
}
