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

// fallback is, as the name suggests, a fallback api.UpstreamServer implementation.
// Any method not implemented in a given RPC client will terminate here.
type fallback struct{}

type completeServer interface {
	api.UpstreamServer
	// An RPC server implementation must closeable by the client.
	io.Closer
}

var _ completeServer = fallback{}

func (fallback) Version(context.Context) (_ string, err error) {
	err = upgradeError("Version")
	return
}

func (fallback) Ping(context.Context) (err error) {
	err = upgradeError("Ping")
	return
}

func (fallback) Export(context.Context) (_ []byte, err error) {
	err = upgradeError("Export")
	return
}

func (fallback) ListServices(context.Context, string) (_ []v6.ControllerStatus, err error) {
	err = upgradeError("ListServices")
	return
}

func (fallback) ListImages(context.Context, update.ResourceSpec) (_ []v6.ImageStatus, err error) {
	err = upgradeError("ListImages")
	return
}

func (fallback) UpdateManifests(context.Context, update.Spec) (_ job.ID, err error) {
	err = upgradeError("UpdateManifests")
	return
}

func (fallback) NotifyChange(context.Context, v9.Change) (err error) {
	err = upgradeError("NotifyChange")
	return
}

func (fallback) JobStatus(context.Context, job.ID) (_ job.Status, err error) {
	err = upgradeError("JobStatus")
	return
}

func (fallback) SyncStatus(context.Context, string) (_ []string, err error) {
	err = upgradeError("SyncStatus")
	return
}

func (fallback) GitRepoConfig(context.Context, bool) (_ v6.GitConfig, err error) {
	err = upgradeError("SyncStatus")
	return
}

func (fallback) Close() (err error) {
	err = upgradeError("Close")
	return
}

func upgradeError(method string) error {
	return remote.UpgradeNeededError(errors.New(method + " method not implemented"))
}
