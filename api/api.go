package api

import (
	"github.com/weaveworks/fluxy"
	"github.com/weaveworks/fluxy/platform"
)

type ClientService interface {
	ListServices(inst flux.InstanceID, namespace string) ([]flux.ServiceStatus, error)
	ListImages(flux.InstanceID, flux.ServiceSpec) ([]flux.ImageStatus, error)
	PostRelease(flux.InstanceID, flux.ReleaseJobSpec) (flux.ReleaseID, error)
	GetRelease(flux.InstanceID, flux.ReleaseID) (flux.ReleaseJob, error)
	Automate(flux.InstanceID, flux.ServiceID) error
	Deautomate(flux.InstanceID, flux.ServiceID) error
	Lock(flux.InstanceID, flux.ServiceID) error
	Unlock(flux.InstanceID, flux.ServiceID) error
	History(flux.InstanceID, flux.ServiceSpec) ([]flux.HistoryEntry, error)
	GetConfig(_ flux.InstanceID, secrets bool) (flux.InstanceConfig, error)
	SetConfig(flux.InstanceID, flux.InstanceConfig) error
}

type DaemonService interface {
	RegisterDaemon(flux.InstanceID, platform.Platform) error
}

type FluxService interface {
	ClientService
	DaemonService
}
