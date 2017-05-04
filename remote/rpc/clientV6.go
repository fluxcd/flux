package rpc

import (
	"io"
	"net/rpc"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/update"
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

func (p *RPCClientV6) ListImages(spec update.ServiceSpec) ([]flux.ImageStatus, error) {
	var images []flux.ImageStatus
	err := p.client.Call("RPCServer.ListImages", spec, &images)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return nil, remote.FatalError{err}
	}
	return images, err
}

func (p *RPCClientV6) UpdateManifests(u update.Spec) (job.ID, error) {
	var result job.ID
	err := p.client.Call("RPCServer.UpdateManifests", u, &result)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return result, remote.FatalError{err}
	}
	return result, err
}

func (p *RPCClientV6) SyncNotify() error {
	var result struct{}
	err := p.client.Call("RPCServer.SyncNotify", struct{}{}, &result)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return remote.FatalError{err}
	}
	return err
}

func (p *RPCClientV6) JobStatus(jobID job.ID) (job.Status, error) {
	var result job.Status
	err := p.client.Call("RPCServer.JobStatus", jobID, &result)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return job.Status{}, remote.FatalError{err}
	}
	return result, err
}

func (p *RPCClientV6) SyncStatus(ref string) ([]string, error) {
	var result []string
	err := p.client.Call("RPCServer.SyncStatus", ref, &result)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return nil, remote.FatalError{err}
	}
	return result, err
}
