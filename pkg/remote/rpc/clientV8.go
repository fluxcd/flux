package rpc

import (
	"context"
	"io"
	"net/rpc"

	"github.com/fluxcd/flux/pkg/api/v10"
	"github.com/fluxcd/flux/pkg/api/v11"
	"github.com/fluxcd/flux/pkg/api/v6"
	"github.com/fluxcd/flux/pkg/job"
	"github.com/fluxcd/flux/pkg/remote"
	"github.com/fluxcd/flux/pkg/update"
)

// RPCClient is the rpc-backed implementation of a server, for
// talking to remote daemons. Version 8 has the same methods, but
// supports a different set of resource kinds to earlier versions.
type RPCClientV8 struct {
	*RPCClientV7
}

type clientV8 interface {
	v6.Server
	v6.Upstream
}

var _ clientV8 = &RPCClientV8{}

var supportedKindsV8 = []string{"deployment", "daemonset", "statefulset", "cronjob", "fluxhelmrelease", "helmrelease"}

// NewClient creates a new rpc-backed implementation of the server.
func NewClientV8(conn io.ReadWriteCloser) *RPCClientV8 {
	return &RPCClientV8{NewClientV7(conn)}
}

func (p *RPCClientV8) ListServicesWithOptions(ctx context.Context, opts v11.ListServicesOptions) ([]v6.ControllerStatus, error) {
	return listServicesWithOptions(ctx, p, opts, supportedKindsV8)
}

func (p *RPCClientV8) ListImages(ctx context.Context, spec update.ResourceSpec) ([]v6.ImageStatus, error) {
	var resp ListImagesResponse
	if err := requireServiceSpecKinds(spec, supportedKindsV8); err != nil {
		return resp.Result, remote.UnsupportedResourceKind(err)
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

func (p *RPCClientV8) ListImagesWithOptions(ctx context.Context, opts v10.ListImagesOptions) ([]v6.ImageStatus, error) {
	return listImagesWithOptions(ctx, p, opts)
}

func (p *RPCClientV8) UpdateManifests(ctx context.Context, u update.Spec) (job.ID, error) {
	var resp UpdateManifestsResponse
	if err := requireSpecKinds(u, supportedKindsV8); err != nil {
		return resp.Result, remote.UnsupportedResourceKind(err)
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
