package rpc

import (
	"context"
	"io"
	"net/rpc"

	"github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/remote"
)

type RPCClientV9 struct {
	*RPCClientV8
}

func NewClientV9(conn io.ReadWriteCloser) *RPCClientV9 {
	return &RPCClientV9{NewClientV8(conn)}
}

func (p *RPCClientV9) NotifyChange(ctx context.Context, c api.Change) error {
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
