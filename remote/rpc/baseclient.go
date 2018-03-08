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

func (bc baseClient) Version(context.Context) (_ string, err error) {
	err = upgradeError("Version")
	return
}

func (bc baseClient) Ping(context.Context) (err error) {
	err = upgradeError("Ping")
	return
}

func (bc baseClient) Export(context.Context) (_ []byte, err error) {
	err = upgradeError("Export")
	return
}

func (bc baseClient) ListServices(context.Context, string) (_ []v6.ControllerStatus, err error) {
	err = upgradeError("ListServices")
	return
}

func (bc baseClient) ListImages(context.Context, update.ResourceSpec) (_ []v6.ImageStatus, err error) {
	err = upgradeError("ListImages")
	return
}

func (bc baseClient) UpdateManifests(context.Context, update.Spec) (_ job.ID, err error) {
	err = upgradeError("UpdateManifests")
	return
}

func (bc baseClient) NotifyChange(context.Context, v9.Change) (err error) {
	err = upgradeError("NotifyChange")
	return
}

func (bc baseClient) JobStatus(context.Context, job.ID) (_ job.Status, err error) {
	err = upgradeError("JobStatus")
	return
}

func (bc baseClient) SyncStatus(context.Context, string) (_ []string, err error) {
	err = upgradeError("SyncStatus")
	return
}

func (bc baseClient) GitRepoConfig(context.Context, bool) (_ v6.GitConfig, err error) {
	err = upgradeError("SyncStatus")
	return
}

func (bc baseClient) Close() (err error) {
	err = upgradeError("Close")
	return
}

func upgradeError(method string) error {
	return remote.UpgradeNeededError(errors.New(method + " method not implemented"))
}
