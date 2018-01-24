package remote

import (
	"context"

	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/update"
)

type ErrorLoggingPlatform struct {
	server api.Server
	logger log.Logger
}

func NewErrorLoggingPlatform(s api.Server, l log.Logger) *ErrorLoggingPlatform {
	return &ErrorLoggingPlatform{s, l}
}

func (p *ErrorLoggingPlatform) Ping(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "Ping", "error", err)
		}
	}()
	return p.server.Ping(ctx)
}

func (p *ErrorLoggingPlatform) Version(ctx context.Context) (v string, err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "Version", "error", err, "version", v)
		}
	}()
	return p.server.Version(ctx)
}

func (p *ErrorLoggingPlatform) Export(ctx context.Context) (config []byte, err error) {
	defer func() {
		if err != nil {
			// Omit config as it could be large
			p.logger.Log("method", "Export", "error", err)
		}
	}()
	return p.server.Export(ctx)
}

func (p *ErrorLoggingPlatform) ListServices(ctx context.Context, maybeNamespace string) (_ []flux.ControllerStatus, err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "ListServices", "error", err)
		}
	}()
	return p.server.ListServices(ctx, maybeNamespace)
}

func (p *ErrorLoggingPlatform) ListImages(ctx context.Context, spec update.ResourceSpec) (_ []flux.ImageStatus, err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "ListImages", "error", err)
		}
	}()
	return p.server.ListImages(ctx, spec)
}

func (p *ErrorLoggingPlatform) NotifyChange(ctx context.Context, change api.Change) (err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "NotifyChange", "error", err)
		}
	}()
	return p.server.NotifyChange(ctx, change)
}

func (p *ErrorLoggingPlatform) JobStatus(ctx context.Context, jobID job.ID) (_ job.Status, err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "JobStatus", "error", err)
		}
	}()
	return p.server.JobStatus(ctx, jobID)
}

func (p *ErrorLoggingPlatform) SyncStatus(ctx context.Context, ref string) (_ []string, err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "SyncStatus", "error", err)
		}
	}()
	return p.server.SyncStatus(ctx, ref)
}

func (p *ErrorLoggingPlatform) UpdateManifests(ctx context.Context, u update.Spec) (_ job.ID, err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "UpdateManifests", "error", err)
		}
	}()
	return p.server.UpdateManifests(ctx, u)
}

func (p *ErrorLoggingPlatform) GitRepoConfig(ctx context.Context, regenerate bool) (_ flux.GitConfig, err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "GitRepoConfig", "error", err)
		}
	}()
	return p.server.GitRepoConfig(ctx, regenerate)
}
