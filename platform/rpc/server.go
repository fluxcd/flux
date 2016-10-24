package rpc

import (
	"errors"
	"io"
	"net/rpc"
	"net/rpc/jsonrpc"

	"github.com/weaveworks/fluxy"
	"github.com/weaveworks/fluxy/platform"
)

// RPCPlatform is the rpc-backed implementation of a platform. For talking to
// remote daemons.
type RPCPlatform struct {
	client *rpc.Client
}

// Platform creates a new rpc-backed implementation of the platform.
func Platform(conn io.ReadWriteCloser) *RPCPlatform {
	return &RPCPlatform{jsonrpc.NewClient(conn)}
}

// Ping, is used to check if the remote platform is available. Might go away,
// and just rely on an error from the other methods.
func (p *RPCPlatform) Ping() error {
	return p.client.Call("RPCClientPlatform.Ping", struct{}{}, nil)
}

// AllServicesRequest is the request datastructure for AllServices
type AllServicesRequest struct {
	MaybeNamespace string
	Ignored        flux.ServiceIDSet
}

// AllServices asks the remote platform to list all services.
func (p *RPCPlatform) AllServices(maybeNamespace string, ignored flux.ServiceIDSet) ([]platform.Service, error) {
	var s []platform.Service
	err := p.client.Call("RPCClientPlatform.AllServices", AllServicesRequest{maybeNamespace, ignored}, &s)
	return s, err
}

// SomeServices asks the remote platform about some specific set of services.
func (p *RPCPlatform) SomeServices(ids []flux.ServiceID) ([]platform.Service, error) {
	var s []platform.Service
	err := p.client.Call("RPCClientPlatform.SomeServices", ids, &s)
	return s, err
}

// Regrade tells the remote platform to apply some regrade specs.
func (p *RPCPlatform) Regrade(spec []platform.RegradeSpec) error {
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
func (p *RPCPlatform) Close() error {
	return p.client.Close()
}
