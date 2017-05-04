package remote

import (
	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/update"
)

type ErrorLoggingPlatform struct {
	Platform Platform
	Logger   log.Logger
}

func (p *ErrorLoggingPlatform) Ping() (err error) {
	defer func() {
		if err != nil {
			p.Logger.Log("method", "Ping", "error", err)
		}
	}()
	return p.Platform.Ping()
}

func (p *ErrorLoggingPlatform) Version() (v string, err error) {
	defer func() {
		if err != nil {
			p.Logger.Log("method", "Version", "error", err, "version", v)
		}
	}()
	return p.Platform.Version()
}

func (p *ErrorLoggingPlatform) Export() (config []byte, err error) {
	defer func() {
		if err != nil {
			// Omit config as it could be large
			p.Logger.Log("method", "Export", "error", err)
		}
	}()
	return p.Platform.Export()
}

func (p *ErrorLoggingPlatform) ListServices(maybeNamespace string) (_ []flux.ServiceStatus, err error) {
	defer func() {
		if err != nil {
			p.Logger.Log("method", "ListServices", "error", err)
		}
	}()
	return p.Platform.ListServices(maybeNamespace)
}

func (p *ErrorLoggingPlatform) ListImages(spec update.ServiceSpec) (_ []flux.ImageStatus, err error) {
	defer func() {
		if err != nil {
			p.Logger.Log("method", "ListImages", "error", err)
		}
	}()
	return p.Platform.ListImages(spec)
}

func (p *ErrorLoggingPlatform) SyncNotify() (err error) {
	defer func() {
		if err != nil {
			p.Logger.Log("method", "SyncNotify", "error", err)
		}
	}()
	return p.Platform.SyncNotify()
}

func (p *ErrorLoggingPlatform) JobStatus(jobID job.ID) (_ job.Status, err error) {
	defer func() {
		if err != nil {
			p.Logger.Log("method", "JobStatus", "error", err)
		}
	}()
	return p.Platform.JobStatus(jobID)
}

func (p *ErrorLoggingPlatform) SyncStatus(rev string) (_ []string, err error) {
	defer func() {
		if err != nil {
			p.Logger.Log("method", "SyncStatus", "error", err)
		}
	}()
	return p.Platform.SyncStatus(rev)
}

func (p *ErrorLoggingPlatform) UpdateManifests(u update.Spec) (_ job.ID, err error) {
	defer func() {
		if err != nil {
			p.Logger.Log("method", "UpdateManifests", "error", err)
		}
	}()
	return p.Platform.UpdateManifests(u)
}
