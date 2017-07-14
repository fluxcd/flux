package api

import (
	"time"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/history"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/service"
	"github.com/weaveworks/flux/ssh"
	"github.com/weaveworks/flux/update"
)

// API for clients connecting to the service.
type ClientService interface {
	Status(inst service.InstanceID) (service.Status, error)
	ListServices(inst service.InstanceID, namespace string) ([]flux.ServiceStatus, error)
	ListImages(service.InstanceID, update.ServiceSpec) ([]flux.ImageStatus, error)
	UpdateImages(service.InstanceID, update.ReleaseSpec, update.Cause) (job.ID, error)
	SyncNotify(service.InstanceID) error
	JobStatus(service.InstanceID, job.ID) (job.Status, error)
	SyncStatus(service.InstanceID, string) ([]string, error)
	UpdatePolicies(service.InstanceID, policy.Updates, update.Cause) (job.ID, error)
	History(service.InstanceID, update.ServiceSpec, time.Time, int64, time.Time) ([]history.Entry, error)
	GetConfig(_ service.InstanceID, fingerprint string) (service.InstanceConfig, error)
	SetConfig(service.InstanceID, service.InstanceConfig) error
	PatchConfig(service.InstanceID, service.ConfigPatch) error
	Export(inst service.InstanceID) ([]byte, error)
	PublicSSHKey(inst service.InstanceID, regenerate bool) (ssh.PublicKey, error)
}

// API for daemons connecting to the service
type DaemonService interface {
	RegisterDaemon(service.InstanceID, remote.Platform) error
	IsDaemonConnected(service.InstanceID) error
	LogEvent(service.InstanceID, history.Event) error
}

type FluxService interface {
	ClientService
	DaemonService
}
