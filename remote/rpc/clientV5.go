package rpc

import (
	"context"
	"io"
	"net/rpc"

	"github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/remote"
)

// RPCClient is the rpc-backed implementation of a platform, for
// talking to remote daemons.
type RPCClientV5 struct {
	*RPCClientV4
}

var _ api.ServerV5 = &RPCClientV5{}

// NewClient creates a new rpc-backed implementation of the platform.
func NewClientV5(conn io.ReadWriteCloser) *RPCClientV5 {
	return &RPCClientV5{NewClientV4(conn)}
}

// Export is used to get service configuration in platform-specific format
func (p *RPCClientV5) Export(ctx context.Context) ([]byte, error) {
	var config []byte
	err := p.client.Call("RPCServer.Export", struct{}{}, &config)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return nil, remote.FatalError{err}
	}
	return config, err
}
