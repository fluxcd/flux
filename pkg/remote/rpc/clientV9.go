package rpc

import (
	"context"
	"io"
	"net/rpc"

	"github.com/fluxcd/flux/pkg/api/v9"
	"github.com/fluxcd/flux/pkg/remote"
)

type RPCClientV9 struct {
	*RPCClientV8
}

type clientV9 interface {
	v9.Server
	v9.Upstream
}

var _ clientV9 = &RPCClientV9{}

func NewClientV9(conn io.ReadWriteCloser) *RPCClientV9 {
	return &RPCClientV9{NewClientV8(conn)}
}

func (p *RPCClientV9) NotifyChange(ctx context.Context, c v9.Change) error {
	var resp NotifyChangeResponse
	err := p.client.Call("RPCServer.NotifyChange", c, &resp)
	if err != nil {
		if _, ok := err.(rpc.ServerError); !ok && err != nil {
			err = remote.FatalError{err}
		}
	} else if resp.ApplicationError != nil {
		err = resp.ApplicationError
	}
	return err
}
