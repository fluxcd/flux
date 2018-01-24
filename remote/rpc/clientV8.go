package rpc

import (
	"context"
	"io"
	"net/rpc"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/update"
)

// RPCClient is the rpc-backed implementation of a server, for
// talking to remote daemons. Version 8 has the same methods, but
// supports a different set of resource kinds to earlier versions.
type RPCClientV8 struct {
	*RPCClientV7
}

var _ api.ServerV6 = &RPCClientV8{}

var supportedKindsV8 = []string{"deployment", "daemonset", "statefulset", "cronjob"}

// NewClient creates a new rpc-backed implementation of the server.
func NewClientV8(conn io.ReadWriteCloser) *RPCClientV8 {
	return &RPCClientV8{NewClientV7(conn)}
}

func (p *RPCClientV8) ListImages(ctx context.Context, spec update.ResourceSpec) ([]flux.ImageStatus, error) {
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
