package rpc

import (
	"io"
	"net/rpc"

	"github.com/weaveworks/flux/platform"
)

// RPCClient is the rpc-backed implementation of a platform, for
// talking to remote daemons.
type RPCClient struct {
	*RPCClientV0
}

// NewClient creates a new rpc-backed implementation of the platform.
func NewClient(conn io.ReadWriteCloser) *RPCClient {
	return &RPCClient{NewClientV0(conn)}
}

// Version is used to check if the remote platform is available
func (p *RPCClient) Version() (string, error) {
	var version string
	err := p.client.Call("RPCServer.Version", struct{}{}, &version)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return "", platform.FatalError{err}
	}
	return version, err
}
