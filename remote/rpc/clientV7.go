package rpc

import (
	"context"
	"io"
	"net/rpc"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/api"
	fluxerr "github.com/weaveworks/flux/errors"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/update"
)

// RPCClient is the rpc-backed implementation of a platform, for
// talking to remote daemons. Version 7 has the same methods, but
// transmits error data properly. The reason it needs a new version is
// that the responses must be decoded differently.
type RPCClientV7 struct {
	*RPCClientV6
}

var _ api.ServerV6 = &RPCClientV7{}

var supportedKindsV7 = []string{"service"}

// NewClient creates a new rpc-backed implementation of the platform.
func NewClientV7(conn io.ReadWriteCloser) *RPCClientV7 {
	return &RPCClientV7{NewClientV6(conn)}
}

// Export is used to get service configuration in platform-specific format
func (p *RPCClientV7) Export(ctx context.Context) ([]byte, error) {
	var resp ExportResponse
	err := p.client.Call("RPCServer.Export", struct{}{}, &resp)
	if err != nil {
		if _, ok := err.(rpc.ServerError); !ok && err != nil {
			return resp.Result, remote.FatalError{err}
		}
		return resp.Result, err
	}
	err = resp.ApplicationError
	return resp.Result, err
}

func (p *RPCClientV7) ListServices(ctx context.Context, namespace string) ([]flux.ControllerStatus, error) {
	var resp ListServicesResponse
	err := p.client.Call("RPCServer.ListServices", namespace, &resp)
	if err != nil {
		if _, ok := err.(rpc.ServerError); !ok && err != nil {
			return resp.Result, remote.FatalError{err}
		}
		return resp.Result, err
	} else if resp.ApplicationError != nil {
		err = resp.ApplicationError
	}
	return resp.Result, err
}

func (p *RPCClientV7) ListImages(ctx context.Context, spec update.ResourceSpec) ([]flux.ImageStatus, error) {
	var resp ListImagesResponse
	if err := requireServiceSpecKinds(spec, supportedKindsV7); err != nil {
		return resp.Result, remote.UpgradeNeededError(err)
	}

	err := p.client.Call("RPCServer.ListImages", spec, &resp)
	if err != nil {
		if _, ok := err.(rpc.ServerError); !ok && err != nil {
			err = remote.FatalError{err}
		}
	} else if resp.ApplicationError != nil {
		err = resp.ApplicationError
	}
	return resp.Result, err
}

func (p *RPCClientV7) UpdateManifests(ctx context.Context, u update.Spec) (job.ID, error) {
	var resp UpdateManifestsResponse
	if err := requireSpecKinds(u, supportedKindsV7); err != nil {
		return resp.Result, remote.UpgradeNeededError(err)
	}
	err := p.client.Call("RPCServer.UpdateManifests", u, &resp)
	if err != nil {
		if _, ok := err.(rpc.ServerError); !ok && err != nil {
			err = remote.FatalError{err}
		}
	} else if resp.ApplicationError != nil {
		err = resp.ApplicationError
	}
	return resp.Result, err
}

type SyncNotifyResponse struct {
	ApplicationError *fluxerr.Error
}

func (p *RPCClientV7) SyncNotify(ctx context.Context) error {
	var resp SyncNotifyResponse
	err := p.client.Call("RPCServer.SyncNotify", struct{}{}, &resp)
	if err != nil {
		if _, ok := err.(rpc.ServerError); !ok && err != nil {
			err = remote.FatalError{err}
		}
	} else if resp.ApplicationError != nil {
		err = resp.ApplicationError
	}
	return err
}

func (p *RPCClientV7) JobStatus(ctx context.Context, jobID job.ID) (job.Status, error) {
	var resp JobStatusResponse
	err := p.client.Call("RPCServer.JobStatus", jobID, &resp)
	if err != nil {
		if _, ok := err.(rpc.ServerError); !ok && err != nil {
			err = remote.FatalError{err}
		}
	} else if resp.ApplicationError != nil {
		err = resp.ApplicationError
	}
	return resp.Result, err
}

func (p *RPCClientV7) SyncStatus(ctx context.Context, ref string) ([]string, error) {
	var resp SyncStatusResponse
	err := p.client.Call("RPCServer.SyncStatus", ref, &resp)
	if err != nil {
		if _, ok := err.(rpc.ServerError); !ok && err != nil {
			err = remote.FatalError{err}
		}
	} else if resp.ApplicationError != nil {
		err = resp.ApplicationError
	}
	return resp.Result, err
}

func (p *RPCClientV7) GitRepoConfig(ctx context.Context, regenerate bool) (flux.GitConfig, error) {
	var resp GitRepoConfigResponse
	err := p.client.Call("RPCServer.GitRepoConfig", regenerate, &resp)
	if err != nil {
		if _, ok := err.(rpc.ServerError); !ok && err != nil {
			err = remote.FatalError{err}
		}
	} else if resp.ApplicationError != nil {
		err = resp.ApplicationError
	}
	return resp.Result, err
}
