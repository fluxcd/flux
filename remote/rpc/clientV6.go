package rpc

import (
	"context"
	"io"
	"net/rpc"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/api"
	fluxerr "github.com/weaveworks/flux/errors"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/update"
)

// RPCClient is the rpc-backed implementation of a platform, for
// talking to remote daemons.
type RPCClientV6 struct {
	*RPCClientV5
}

// We don't get proper application error structs back from v6, but we
// do know that anything that's not considered a fatal error can be
// translated into an application error.
func remoteApplicationError(err error) error {
	return &fluxerr.Error{
		Type: fluxerr.User,
		Err:  err,
		Help: `Error from daemon

The daemon (fluxd, running in your cluster) reported this error when
attempting to fulfil your request:

    ` + err.Error() + `
`,
	}
}

var _ api.ServerV6 = &RPCClientV6{}

var supportedKindsV6 = []string{"service"}

// NewClient creates a new rpc-backed implementation of the platform.
func NewClientV6(conn io.ReadWriteCloser) *RPCClientV6 {
	return &RPCClientV6{NewClientV5(conn)}
}

// Export is used to get service configuration in platform-specific format
func (p *RPCClientV6) Export(ctx context.Context) ([]byte, error) {
	var config []byte
	err := p.client.Call("RPCServer.Export", struct{}{}, &config)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return nil, remote.FatalError{err}
	}
	if err != nil {
		err = remoteApplicationError(err)
	}
	return config, err
}

// Export is used to get service configuration in platform-specific format
func (p *RPCClientV6) ListServices(ctx context.Context, namespace string) ([]flux.ControllerStatus, error) {
	var services []flux.ControllerStatus
	err := p.client.Call("RPCServer.ListServices", namespace, &services)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return nil, remote.FatalError{err}
	}
	if err != nil {
		err = remoteApplicationError(err)
	}
	return services, err
}

func (p *RPCClientV6) ListImages(ctx context.Context, spec update.ResourceSpec) ([]flux.ImageStatus, error) {
	var images []flux.ImageStatus
	if err := requireServiceSpecKinds(spec, supportedKindsV6); err != nil {
		return images, remote.UpgradeNeededError(err)
	}

	err := p.client.Call("RPCServer.ListImages", spec, &images)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return nil, remote.FatalError{err}
	}
	if err != nil {
		err = remoteApplicationError(err)
	}
	return images, err
}

func (p *RPCClientV6) UpdateManifests(ctx context.Context, u update.Spec) (job.ID, error) {
	var result job.ID
	if err := requireSpecKinds(u, supportedKindsV6); err != nil {
		return result, remote.UpgradeNeededError(err)
	}

	err := p.client.Call("RPCServer.UpdateManifests", u, &result)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return result, remote.FatalError{err}
	}
	if err != nil {
		err = remoteApplicationError(err)
	}
	return result, err
}

func (p *RPCClientV6) SyncNotify(ctx context.Context) error {
	var result struct{}
	err := p.client.Call("RPCServer.SyncNotify", struct{}{}, &result)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return remote.FatalError{err}
	}
	if err != nil {
		err = remoteApplicationError(err)
	}
	return err
}

func (p *RPCClientV6) JobStatus(ctx context.Context, jobID job.ID) (job.Status, error) {
	var result job.Status
	err := p.client.Call("RPCServer.JobStatus", jobID, &result)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return job.Status{}, remote.FatalError{err}
	}
	if err != nil {
		err = remoteApplicationError(err)
	}
	return result, err
}

func (p *RPCClientV6) SyncStatus(ctx context.Context, ref string) ([]string, error) {
	var result []string
	err := p.client.Call("RPCServer.SyncStatus", ref, &result)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return nil, remote.FatalError{err}
	}
	if err != nil {
		err = remoteApplicationError(err)
	}
	return result, err
}

func (p *RPCClientV6) GitRepoConfig(ctx context.Context, regenerate bool) (flux.GitConfig, error) {
	var result flux.GitConfig
	err := p.client.Call("RPCServer.GitRepoConfig", regenerate, &result)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return flux.GitConfig{}, remote.FatalError{err}
	}
	if err != nil {
		err = remoteApplicationError(err)
	}
	return result, err
}
