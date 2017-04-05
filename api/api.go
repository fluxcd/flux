package api

import (
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/jobs"
	"github.com/weaveworks/flux/platform"
)

type ClientService interface {
	Status(inst flux.InstanceID) (flux.Status, error)
	ListServices(inst flux.InstanceID, namespace string) ([]flux.ServiceStatus, error)
	ListImages(flux.InstanceID, flux.ServiceSpec) ([]flux.ImageStatus, error)
	PostRelease(flux.InstanceID, jobs.ReleaseJobParams) (jobs.JobID, error)
	GetRelease(flux.InstanceID, jobs.JobID) (jobs.Job, error)
	Automate(flux.InstanceID, flux.ServiceID) error
	Deautomate(flux.InstanceID, flux.ServiceID) error
	Lock(flux.InstanceID, flux.ServiceID) error
	Unlock(flux.InstanceID, flux.ServiceID) error
	History(flux.InstanceID, flux.ServiceSpec) ([]flux.HistoryEntry, error)
	GetConfig(_ flux.InstanceID) (flux.InstanceConfig, error)
	SetConfig(flux.InstanceID, flux.UnsafeInstanceConfig) error
	PatchConfig(flux.InstanceID, flux.ConfigPatch) error
	GenerateDeployKey(flux.InstanceID) error
	Export(inst flux.InstanceID) ([]byte, error)
}

type DaemonService interface {
	RegisterDaemon(flux.InstanceID, platform.Platform) error
	IsDaemonConnected(flux.InstanceID) error
}

type FluxService interface {
	ClientService
	DaemonService
}
