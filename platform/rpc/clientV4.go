package rpc

import (
	"errors"
	"io"
	"net/rpc"
	"net/rpc/jsonrpc"

	"github.com/weaveworks/flux"
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

// AllServicesRequest is the request datastructure for AllServices
type AllServicesRequestV4 struct {
	MaybeNamespace string
	Ignored        flux.ServiceIDSet
}

// AllServices asks the remote platform to list all services.
func (p *RPCClientV4) AllServices(maybeNamespace string, ignored flux.ServiceIDSet) ([]platform.Service, error) {
	var s []platform.Service
	err := p.client.Call("RPCServer.AllServices", AllServicesRequestV4{maybeNamespace, ignored}, &s)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		err = platform.FatalError{err}
	}
	return s, err
}

// SomeServices asks the remote platform about some specific set of services.
func (p *RPCClientV4) SomeServices(ids []flux.ServiceID) ([]platform.Service, error) {
	var s []platform.Service
	err := p.client.Call("RPCServer.SomeServices", ids, &s)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		err = platform.FatalError{err}
	}
	return s, err
}

// Apply tells the remote platform to apply some new service definitions.
func (p *RPCClientV4) Apply(defs []platform.ServiceDefinition) error {
	var applyErrors ApplyResult
	// TODO: This is still calling "Regrade" for backwards compatibility with old
	// fluxds. Change this to "Apply" when we do a major version release.
	if err := p.client.Call("RPCServer.Regrade", defs, &applyErrors); err != nil {
		if _, ok := err.(rpc.ServerError); !ok && err != nil {
			err = platform.FatalError{err}
		}
		return err
	}
	if len(applyErrors) > 0 {
		errs := platform.ApplyError{}
		for s, e := range applyErrors {
			errs[s] = errors.New(e)
		}
		return errs
	}
	return nil
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
