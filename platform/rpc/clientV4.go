package rpc

import (
	"io"
	"net/rpc"
	"net/rpc/jsonrpc"

	"github.com/weaveworks/flux/platform"
)

// RPCClientV4 is the rpc-backed implementation of a platform, for
// talking to remote daemons.
type RPCClientV4 struct {
	*baseClient
	client *rpc.Client
}

var _ platform.PlatformV4 = &RPCClientV4{}

// NewClient creates a new rpc-backed implementation of the platform.
func NewClientV4(conn io.ReadWriteCloser) *RPCClientV4 {
	return &RPCClientV4{&baseClient{}, jsonrpc.NewClient(conn)}
}

// Ping is used to check if the remote platform is available.
func (p *RPCClientV4) Ping() error {
	err := p.client.Call("RPCServer.Ping", struct{}{}, nil)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return platform.FatalError{err}
	}
	return err
}

// Version is used to check if the remote platform is available
func (p *RPCClientV4) Version() (string, error) {
	var version string
	err := p.client.Call("RPCServer.Version", struct{}{}, &version)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return "", platform.FatalError{err}
	} else if err != nil && err.Error() == "rpc: can't find method RPCServer.Version" {
		// "Version" is not supported by this version of fluxd (it is old). Fail
		// gracefully.
		return "unknown", nil
	}
	return version, err
}

// Close closes the connection to the remote platform, it does *not* cause the
// remote platform to shut down.
func (p *RPCClientV4) Close() error {
	return p.client.Close()
}
