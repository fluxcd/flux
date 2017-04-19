package rpc

import (
	"errors"
	"io"
	"net/rpc"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/remote"
)

// RPCClient is the rpc-backed implementation of a platform, for
// talking to remote daemons.
type RPCClientV6 struct {
	*RPCClientV5
}

var _ remote.PlatformV6 = &RPCClientV6{}

// NewClient creates a new rpc-backed implementation of the platform.
func NewClientV6(conn io.ReadWriteCloser) *RPCClientV6 {
	return &RPCClientV6{NewClientV5(conn)}
}

// Export is used to get service configuration in platform-specific format
func (p *RPCClientV6) Export() ([]byte, error) {
	var config []byte
	err := p.client.Call("RPCServer.Export", struct{}{}, &config)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return nil, remote.FatalError{err}
	}
	return config, err
}

// Export is used to get service configuration in platform-specific format
func (p *RPCClientV6) ListServices(namespace string) ([]flux.ServiceStatus, error) {
	var services []flux.ServiceStatus
	err := p.client.Call("RPCServer.ListServices", namespace, &services)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return nil, remote.FatalError{err}
	}
	return services, err
}

func (p *RPCClientV6) ListImages(spec flux.ServiceSpec) ([]flux.ImageStatus, error) {
	var images []flux.ImageStatus
	err := p.client.Call("RPCServer.ListImages", spec, &images)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return nil, remote.FatalError{err}
	}
	return images, err
}

func (p *RPCClientV6) UpdateImages(spec flux.ReleaseSpec) (flux.ReleaseResult, error) {
	return nil, errors.New("FIXME")
}

func (p *RPCClientV6) SyncCluster() error {
	return errors.New("FIXME")
}

func (p *RPCClientV6) SyncStatus(cursor string) ([]string, error) {
	return nil, errors.New("FIXME")
}
