package rpc

import (
	"context"
	"io"
	"net/rpc"

	"github.com/fluxcd/flux/pkg/api/v10"
	"github.com/fluxcd/flux/pkg/api/v11"
	"github.com/fluxcd/flux/pkg/api/v6"
	fluxerr "github.com/fluxcd/flux/pkg/errors"
	"github.com/fluxcd/flux/pkg/job"
	"github.com/fluxcd/flux/pkg/remote"
	"github.com/fluxcd/flux/pkg/update"
)

// RPCClient is the rpc-backed implementation of a server, for
// talking to remote daemons. Version 7 has the same methods, but
// transmits error data properly. The reason it needs a new version is
// that the responses must be decoded differently.
type RPCClientV7 struct {
	*RPCClientV6
}

type clientV7 interface {
	v6.Server
	v6.Upstream
}

var _ clientV7 = &RPCClientV7{}

var supportedKindsV7 = []string{"service"}

// NewClient creates a new rpc-backed implementation of the server.
func NewClientV7(conn io.ReadWriteCloser) *RPCClientV7 {
	return &RPCClientV7{NewClientV6(conn)}
}

// Export is used to get service configuration in cluster-specific format
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

func (p *RPCClientV7) ListServices(ctx context.Context, namespace string) ([]v6.ControllerStatus, error) {
	var resp ListServicesResponse
	err := p.client.Call("RPCServer.ListServices", namespace, &resp)
	listServicesRolloutStatus(resp.Result)
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

func (p *RPCClientV7) ListServicesWithOptions(ctx context.Context, opts v11.ListServicesOptions) ([]v6.ControllerStatus, error) {
	return listServicesWithOptions(ctx, p, opts, supportedKindsV7)
}

func (p *RPCClientV7) ListImages(ctx context.Context, spec update.ResourceSpec) ([]v6.ImageStatus, error) {
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

func (p *RPCClientV7) ListImagesWithOptions(ctx context.Context, opts v10.ListImagesOptions) ([]v6.ImageStatus, error) {
	return listImagesWithOptions(ctx, p, opts)
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

func (p *RPCClientV7) GitRepoConfig(ctx context.Context, regenerate bool) (v6.GitConfig, error) {
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
