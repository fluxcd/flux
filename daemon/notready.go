package daemon

import (
	"sync"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/update"
)

// NotReadyDaemon is a stub implementation used to serve a subset of the
// API when we have yet to successfully clone the config repo.
type NotReadyDaemon struct {
	sync.RWMutex
	version   string
	cluster   cluster.Cluster
	gitRemote flux.GitRemoteConfig
	reason    error
}

func NewNotReadyDaemon(version string, cluster cluster.Cluster, gitRemote flux.GitRemoteConfig, reason error) (nrd *NotReadyDaemon) {
	return &NotReadyDaemon{
		version:   version,
		cluster:   cluster,
		gitRemote: gitRemote,
		reason:    reason,
	}
}

func (nrd *NotReadyDaemon) Reason() error {
	nrd.RLock()
	defer nrd.RUnlock()
	return nrd.reason
}

func (nrd *NotReadyDaemon) UpdateReason(reason error) {
	nrd.Lock()
	nrd.reason = reason
	nrd.Unlock()
}

// 'Not ready' platform implementation

func (nrd *NotReadyDaemon) Ping() error {
	return nrd.cluster.Ping()
}

func (nrd *NotReadyDaemon) Version() (string, error) {
	return nrd.version, nil
}

func (nrd *NotReadyDaemon) Export() ([]byte, error) {
	return nrd.cluster.Export()
}

func (nrd *NotReadyDaemon) ListServices(namespace string) ([]flux.ServiceStatus, error) {
	return nil, nrd.Reason()
}

func (nrd *NotReadyDaemon) ListImages(update.ServiceSpec) ([]flux.ImageStatus, error) {
	return nil, nrd.Reason()
}

func (nrd *NotReadyDaemon) UpdateManifests(update.Spec) (job.ID, error) {
	var id job.ID
	return id, nrd.Reason()
}

func (nrd *NotReadyDaemon) SyncNotify() error {
	return nrd.Reason()
}

func (nrd *NotReadyDaemon) JobStatus(id job.ID) (job.Status, error) {
	return job.Status{}, nrd.Reason()
}

func (nrd *NotReadyDaemon) SyncStatus(string) ([]string, error) {
	return nil, nrd.Reason()
}

func (nrd *NotReadyDaemon) GitRepoConfig(regenerate bool) (flux.GitConfig, error) {
	publicSSHKey, err := nrd.cluster.PublicSSHKey(regenerate)
	if err != nil {
		return flux.GitConfig{}, err
	}
	return flux.GitConfig{
		Remote:       nrd.gitRemote,
		PublicSSHKey: publicSSHKey,
	}, nil
}
