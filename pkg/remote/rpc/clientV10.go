package rpc

import (
	"context"
	"io"
	"net/rpc"

	"github.com/fluxcd/flux/pkg/api/v10"
	"github.com/fluxcd/flux/pkg/api/v6"
	"github.com/fluxcd/flux/pkg/remote"
)

// RPCClientV10 is the rpc-backed implementation of a server, for
// talking to remote daemons. This version introduces methods which accept an
// options struct as the first argument. e.g. ListImagesWithOptions
type RPCClientV10 struct {
	*RPCClientV9
}

type clientV10 interface {
	v10.Server
	v10.Upstream
}

var _ clientV10 = &RPCClientV10{}

// NewClientV10 creates a new rpc-backed implementation of the server.
func NewClientV10(conn io.ReadWriteCloser) *RPCClientV10 {
	return &RPCClientV10{NewClientV9(conn)}
}

func (p *RPCClientV10) ListImagesWithOptions(ctx context.Context, opts v10.ListImagesOptions) ([]v6.ImageStatus, error) {
	var resp ListImagesResponse
	if err := requireServiceSpecKinds(opts.Spec, supportedKindsV8); err != nil {
		return resp.Result, remote.UnsupportedResourceKind(err)
	}

	err := p.client.Call("RPCServer.ListImagesWithOptions", opts, &resp)
	if err != nil {
		if _, ok := err.(rpc.ServerError); !ok && err != nil {
			err = remote.FatalError{err}
		}
	} else if resp.ApplicationError != nil {
		err = resp.ApplicationError
	}
	return resp.Result, err
}
