package rpc

import (
	"errors"
	"io"
	"net/rpc"
	"net/rpc/jsonrpc"

	"github.com/weaveworks/fluxy"
	"github.com/weaveworks/fluxy/platform"
)

// RPCClient is the rpc-backed implementation of a platform, for
// talking to remote daemons.
type RPCClient struct {
	client *rpc.Client
}

// NewClient creates a new rpc-backed implementation of the platform.
func NewClient(conn io.ReadWriteCloser) *RPCClient {
	return &RPCClient{jsonrpc.NewClient(conn)}
}

// Ping, is used to check if the remote platform is available. Might go away,
// and just rely on an error from the other methods.
func (p *RPCClient) Ping() error {
	return p.client.Call("RPCClientPlatform.Ping", struct{}{}, nil)
}

// AllServicesRequest is the request datastructure for AllServices
type AllServicesRequest struct {
	MaybeNamespace string
	Ignored        flux.ServiceIDSet
}

// AllServices asks the remote platform to list all services.
func (p *RPCClient) AllServices(maybeNamespace string, ignored flux.ServiceIDSet) ([]platform.Service, error) {
	var s []platform.Service
	err := p.client.Call("RPCClientPlatform.AllServices", AllServicesRequest{maybeNamespace, ignored}, &s)
	return s, err
}

// SomeServices asks the remote platform about some specific set of services.
func (p *RPCClient) SomeServices(ids []flux.ServiceID) ([]platform.Service, error) {
	var s []platform.Service
	err := p.client.Call("RPCClientPlatform.SomeServices", ids, &s)
	return s, err
}

// Regrade tells the remote platform to apply some regrade specs.
func (p *RPCClient) Regrade(spec []platform.RegradeSpec) error {
	var regradeErrors RegradeResult
	if err := p.client.Call("RPCClientPlatform.Regrade", spec, &regradeErrors); err != nil {
		return err
	}
	if len(regradeErrors) > 0 {
		errs := platform.RegradeError{}
		for s, e := range regradeErrors {
			errs[s] = errors.New(e)
		}
		return errs
	}
	return nil
}

// Close closes the connection to the remote platform, it does *not* cause the
// remote platform to shut down.
func (p *RPCClient) Close() error {
	return p.client.Close()
}
