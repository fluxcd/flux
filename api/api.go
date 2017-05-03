package api

import (
	"time"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/remote"
)

// API for clients connecting to the service.
type ClientService interface {
	Status(inst flux.InstanceID) (flux.Status, error)
	ListServices(inst flux.InstanceID, namespace string) ([]flux.ServiceStatus, error)
	ListImages(flux.InstanceID, flux.ServiceSpec) ([]flux.ImageStatus, error)
	UpdateImages(flux.InstanceID, flux.ReleaseSpec) (job.ID, error)
	SyncNotify(flux.InstanceID) error
	SyncStatus(flux.InstanceID, string) ([]string, error)
	UpdatePolicies(flux.InstanceID, flux.PolicyUpdates) (job.ID, error)
	History(flux.InstanceID, flux.ServiceSpec, time.Time, int64) ([]flux.HistoryEntry, error)
	GetConfig(_ flux.InstanceID, fingerprint string) (flux.InstanceConfig, error)
	SetConfig(flux.InstanceID, flux.UnsafeInstanceConfig) error
	PatchConfig(flux.InstanceID, flux.ConfigPatch) error
	GenerateDeployKey(flux.InstanceID) error
	Export(inst flux.InstanceID) ([]byte, error)
}

// API for daemons connecting to the service
type DaemonService interface {
	RegisterDaemon(flux.InstanceID, remote.Platform) error
	IsDaemonConnected(flux.InstanceID) error
	LogEvent(flux.InstanceID, flux.Event) error
}

type FluxService interface {
	ClientService
	DaemonService
}
