package rpc

import (
	"errors"
	"io"
	"net/rpc"
	"net/rpc/jsonrpc"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/platform"
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

// AllServicesRequest is the request datastructure for AllServices
type AllServicesRequest struct {
	MaybeNamespace string
	Ignored        flux.ServiceIDSet
}

// AllServices asks the remote platform to list all services.
func (p *RPCClient) AllServices(maybeNamespace string, ignored flux.ServiceIDSet) ([]platform.Service, error) {
	var s []platform.Service
	err := p.client.Call("RPCServer.AllServices", AllServicesRequest{maybeNamespace, ignored}, &s)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		err = platform.FatalError{err}
	}
	return s, err
}

// SomeServices asks the remote platform about some specific set of services.
func (p *RPCClient) SomeServices(ids []flux.ServiceID) ([]platform.Service, error) {
	var s []platform.Service
	err := p.client.Call("RPCServer.SomeServices", ids, &s)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		err = platform.FatalError{err}
	}
	return s, err
}

// Release tells the remote platform to apply some release specs.
func (p *RPCClient) Release(spec []platform.ReleaseSpec) error {
	var releaseErrors ReleaseResult
	if err := p.client.Call("RPCServer.Release", spec, &releaseErrors); err != nil {
		if _, ok := err.(rpc.ServerError); !ok && err != nil {
			err = platform.FatalError{err}
		}
		return err
	}
	if len(releaseErrors) > 0 {
		errs := platform.ReleaseError{}
		for s, e := range releaseErrors {
			errs[s] = errors.New(e)
		}
		return errs
	}
	return nil
}

// Ping is used to check if the remote platform is available.
func (p *RPCClient) Ping() error {
	err := p.client.Call("RPCServer.Ping", struct{}{}, nil)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return platform.FatalError{err}
	}
	return err
}

// Close closes the connection to the remote platform, it does *not* cause the
// remote platform to shut down.
func (p *RPCClient) Close() error {
	return p.client.Close()
}
