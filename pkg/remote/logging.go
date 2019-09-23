package remote

import (
	"context"

	"github.com/go-kit/kit/log"

	"github.com/fluxcd/flux/pkg/api"
	"github.com/fluxcd/flux/pkg/api/v10"
	"github.com/fluxcd/flux/pkg/api/v11"
	"github.com/fluxcd/flux/pkg/api/v6"
	"github.com/fluxcd/flux/pkg/api/v9"
	"github.com/fluxcd/flux/pkg/job"
	"github.com/fluxcd/flux/pkg/update"
)

var _ api.Server = &ErrorLoggingServer{}

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

func (p *ErrorLoggingServer) ListServices(ctx context.Context, maybeNamespace string) (_ []v6.ControllerStatus, err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "ListServices", "error", err)
		}
	}()
	return p.server.ListServices(ctx, maybeNamespace)
}

func (p *ErrorLoggingServer) ListServicesWithOptions(ctx context.Context, opts v11.ListServicesOptions) (_ []v6.ControllerStatus, err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "ListServicesWithOptions", "error", err)
		}
	}()
	return p.server.ListServicesWithOptions(ctx, opts)
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

func (p *ErrorLoggingServer) Ping(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "Ping", "error", err)
		}
	}()
	return p.server.Ping(ctx)
}

func (p *ErrorLoggingServer) Version(ctx context.Context) (v string, err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "Version", "error", err, "version", v)
		}
	}()
	return p.server.Version(ctx)
}

func (p *ErrorLoggingServer) NotifyChange(ctx context.Context, change v9.Change) (err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "NotifyChange", "error", err)
		}
	}()
	return p.server.NotifyChange(ctx, change)
}
