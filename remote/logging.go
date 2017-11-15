package remote

import (
	"context"

	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/update"
)

type ErrorLoggingPlatform struct {
	Platform Platform
	Logger   log.Logger
}

func (p *ErrorLoggingPlatform) Ping(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			p.Logger.Log("method", "Ping", "error", err)
		}
	}()
	return p.Platform.Ping(ctx)
}

func (p *ErrorLoggingPlatform) Version(ctx context.Context) (v string, err error) {
	defer func() {
		if err != nil {
			p.Logger.Log("method", "Version", "error", err, "version", v)
		}
	}()
	return p.Platform.Version(ctx)
}

func (p *ErrorLoggingPlatform) Export(ctx context.Context) (config []byte, err error) {
	defer func() {
		if err != nil {
			// Omit config as it could be large
			p.Logger.Log("method", "Export", "error", err)
		}
	}()
	return p.Platform.Export(ctx)
}

func (p *ErrorLoggingPlatform) ListServices(ctx context.Context, maybeNamespace string) (_ []flux.ControllerStatus, err error) {
	defer func() {
		if err != nil {
			p.Logger.Log("method", "ListServices", "error", err)
		}
	}()
	return p.Platform.ListServices(ctx, maybeNamespace)
}

func (p *ErrorLoggingPlatform) ListImages(ctx context.Context, spec update.ResourceSpec) (_ []flux.ImageStatus, err error) {
	defer func() {
		if err != nil {
			p.Logger.Log("method", "ListImages", "error", err)
		}
	}()
	return p.Platform.ListImages(ctx, spec)
}

func (p *ErrorLoggingPlatform) NotifyChange(ctx context.Context, change Change) (err error) {
	defer func() {
		if err != nil {
			p.Logger.Log("method", "NotifyChange", "error", err)
		}
	}()
	return p.Platform.NotifyChange(ctx, change)
}

func (p *ErrorLoggingPlatform) JobStatus(ctx context.Context, jobID job.ID) (_ job.Status, err error) {
	defer func() {
		if err != nil {
			p.Logger.Log("method", "JobStatus", "error", err)
		}
	}()
	return p.Platform.JobStatus(ctx, jobID)
}

func (p *ErrorLoggingPlatform) SyncStatus(ctx context.Context, ref string) (_ []string, err error) {
	defer func() {
		if err != nil {
			p.Logger.Log("method", "SyncStatus", "error", err)
		}
	}()
	return p.Platform.SyncStatus(ctx, ref)
}

func (p *ErrorLoggingPlatform) UpdateManifests(ctx context.Context, u update.Spec) (_ job.ID, err error) {
	defer func() {
		if err != nil {
			p.Logger.Log("method", "UpdateManifests", "error", err)
		}
	}()
	return p.Platform.UpdateManifests(ctx, u)
}

func (p *ErrorLoggingPlatform) GitRepoConfig(ctx context.Context, regenerate bool) (_ flux.GitConfig, err error) {
	defer func() {
		if err != nil {
			p.Logger.Log("method", "GitRepoConfig", "error", err)
		}
	}()
	return p.Platform.GitRepoConfig(ctx, regenerate)
}
