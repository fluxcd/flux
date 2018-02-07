package daemon

import (
	"context"
	"errors"
	"sync"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/api"
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
	gitStatus flux.GitRepoStatus
	reason    error
}

// NotReadyDaemon is a state of the daemon that has not proceeded past
// getting the git repo set up. Since this typically needs some
// actions on the part of the user, this state can last indefinitely;
// so, it has its own code.
func NewNotReadyDaemon(version string, cluster cluster.Cluster, gitRemote flux.GitRemoteConfig) (nrd *NotReadyDaemon) {
	return &NotReadyDaemon{
		version:   version,
		cluster:   cluster,
		gitRemote: gitRemote,
		gitStatus: flux.RepoNoConfig,
		reason:    errors.New("git repo is not configured"),
	}
}

func (nrd *NotReadyDaemon) Reason() error {
	nrd.RLock()
	defer nrd.RUnlock()
	return nrd.reason
}

func (nrd *NotReadyDaemon) UpdateStatus(status flux.GitRepoStatus, reason error) {
	nrd.Lock()
	nrd.gitStatus = status
	nrd.reason = reason
	nrd.Unlock()
}

func (nrd *NotReadyDaemon) Ping(ctx context.Context) error {
	return nrd.cluster.Ping()
}

func (nrd *NotReadyDaemon) Version(ctx context.Context) (string, error) {
	return nrd.version, nil
}

func (nrd *NotReadyDaemon) Export(ctx context.Context) ([]byte, error) {
	return nrd.cluster.Export()
}

func (nrd *NotReadyDaemon) ListServices(ctx context.Context, namespace string) ([]flux.ControllerStatus, error) {
	return nil, nrd.Reason()
}

func (nrd *NotReadyDaemon) ListImages(context.Context, update.ResourceSpec) ([]flux.ImageStatus, error) {
	return nil, nrd.Reason()
}

func (nrd *NotReadyDaemon) UpdateManifests(context.Context, update.Spec) (job.ID, error) {
	var id job.ID
	return id, nrd.Reason()
}

func (nrd *NotReadyDaemon) NotifyChange(context.Context, api.Change) error {
	return nrd.Reason()
}

func (nrd *NotReadyDaemon) JobStatus(context.Context, job.ID) (job.Status, error) {
	return job.Status{}, nrd.Reason()
}

func (nrd *NotReadyDaemon) SyncStatus(context.Context, string) ([]string, error) {
	return nil, nrd.Reason()
}

func (nrd *NotReadyDaemon) GitRepoConfig(ctx context.Context, regenerate bool) (flux.GitConfig, error) {
	publicSSHKey, err := nrd.cluster.PublicSSHKey(regenerate)
	if err != nil {
		return flux.GitConfig{}, err
	}
	nrd.RLock()
	defer nrd.RUnlock()
	return flux.GitConfig{
		Remote:       nrd.gitRemote,
		PublicSSHKey: publicSSHKey,
		Status:       nrd.gitStatus,
	}, nil
}
