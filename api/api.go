package api

import (
	"time"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/history"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/update"
)

// API for clients connecting to the service.
type ClientService interface {
	Status(inst flux.InstanceID) (flux.Status, error)
	ListServices(inst flux.InstanceID, namespace string) ([]flux.ServiceStatus, error)
	ListImages(flux.InstanceID, update.ServiceSpec) ([]flux.ImageStatus, error)
	UpdateImages(flux.InstanceID, update.ReleaseSpec) (job.ID, error)
	SyncNotify(flux.InstanceID) error
	JobStatus(flux.InstanceID, job.ID) (job.Status, error)
	SyncStatus(flux.InstanceID, string) ([]string, error)
	UpdatePolicies(flux.InstanceID, policy.Updates) (job.ID, error)
	History(flux.InstanceID, update.ServiceSpec, time.Time, int64) ([]history.Entry, error)
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
	LogEvent(flux.InstanceID, history.Event) error
}

type FluxService interface {
	ClientService
	DaemonService
}
