package rpc

import (
	"context"
	"io"

	"github.com/pkg/errors"

	"github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/api/v6"
	"github.com/weaveworks/flux/api/v9"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/update"
)

type baseClient struct{}

type completeServer interface {
	api.UpstreamServer
	// An RPC server implementation must closeable by the client.
	io.Closer
}

var _ completeServer = baseClient{}

func (bc baseClient) Version(context.Context) (string, error) {
	return "", remote.UpgradeNeededError(errors.New("Version method not implemented"))
}

func (bc baseClient) Ping(context.Context) error {
	return remote.UpgradeNeededError(errors.New("Ping method not implemented"))
}

func (bc baseClient) Export(context.Context) ([]byte, error) {
	return nil, remote.UpgradeNeededError(errors.New("Export method not implemented"))
}

func (bc baseClient) ListServices(context.Context, string) ([]v6.ControllerStatus, error) {
	return nil, remote.UpgradeNeededError(errors.New("ListServices method not implemented"))
}

func (bc baseClient) ListImages(context.Context, update.ResourceSpec) ([]v6.ImageStatus, error) {
	return nil, remote.UpgradeNeededError(errors.New("ListImages method not implemented"))
}

func (bc baseClient) UpdateManifests(context.Context, update.Spec) (job.ID, error) {
	var id job.ID
	return id, remote.UpgradeNeededError(errors.New("UpdateManifests method not implemented"))
}

func (bc baseClient) NotifyChange(context.Context, v9.Change) error {
	return remote.UpgradeNeededError(errors.New("NotifyChange method not implemented"))
}

func (bc baseClient) JobStatus(context.Context, job.ID) (job.Status, error) {
	return job.Status{}, remote.UpgradeNeededError(errors.New("JobStatus method not implemented"))
}

func (bc baseClient) SyncStatus(context.Context, string) ([]string, error) {
	return nil, remote.UpgradeNeededError(errors.New("SyncStatus method not implemented"))
}

func (bc baseClient) GitRepoConfig(context.Context, bool) (v6.GitConfig, error) {
	return v6.GitConfig{}, remote.UpgradeNeededError(errors.New("GitRepoConfig method not implemented"))
}

func (bc baseClient) Close() error {
	return remote.UpgradeNeededError(errors.New("Close method not implemented"))
}
