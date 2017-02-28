package server

import (
	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/platform"
)

type loggingPlatform struct {
	platform platform.Platform
	logger   log.Logger
}

func (p *loggingPlatform) AllServices(maybeNamespace string, ignored flux.ServiceIDSet) (ss []platform.Service, err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "AllServices", "error", err)
		}
	}()
	return p.platform.AllServices(maybeNamespace, ignored)
}

func (p *loggingPlatform) SomeServices(include []flux.ServiceID) (ss []platform.Service, err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "SomeServices", "error", err)
		}
	}()
	return p.platform.SomeServices(include)
}

func (p *loggingPlatform) Apply(defs []platform.ServiceDefinition) (err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "Apply", "error", err)
		}
	}()
	return p.platform.Apply(defs)
}

func (p *loggingPlatform) Ping() (err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "Ping", "error", err)
		}
	}()
	return p.platform.Ping()
}

func (p *loggingPlatform) Version() (v string, err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "Version", "error", err, "version", v)
		}
	}()
	return p.platform.Version()
}

func (p *loggingPlatform) Export() (config []byte, err error) {
	defer func() {
		if err != nil {
			// Omit config as it could be large
			p.logger.Log("method", "Export", "error", err)
		}
	}()
	return p.platform.Export()
}

func (p *loggingPlatform) Sync(def platform.SyncDef) (err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "Sync", "error", err)
		}
	}()
	return p.platform.Sync(def)
}
