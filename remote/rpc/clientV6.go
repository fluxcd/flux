package rpc

import (
	"context"
	"io"
	"net/rpc"
	"net/rpc/jsonrpc"

	"github.com/weaveworks/flux/api/v10"
	"github.com/weaveworks/flux/api/v11"
	"github.com/weaveworks/flux/api/v6"
	fluxerr "github.com/weaveworks/flux/errors"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/update"
)

// RPCClient is the rpc-backed implementation of a server, for
// talking to remote daemons.
type RPCClientV6 struct {
	*baseClient
	client *rpc.Client
}

type clientV6 interface {
	v6.Server
	v6.Upstream
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

var _ clientV6 = &RPCClientV6{}

var supportedKindsV6 = []string{"service"}

// NewClient creates a new rpc-backed implementation of the server.
func NewClientV6(conn io.ReadWriteCloser) *RPCClientV6 {
	return &RPCClientV6{&baseClient{}, jsonrpc.NewClient(conn)}
}

// Ping is used to check if the remote server is available.
func (p *RPCClientV6) Ping(ctx context.Context) error {
	err := p.client.Call("RPCServer.Ping", struct{}{}, nil)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return remote.FatalError{err}
	}
	return err
}

// Version is used to check the version of the remote server.
func (p *RPCClientV6) Version(ctx context.Context) (string, error) {
	var version string
	err := p.client.Call("RPCServer.Version", struct{}{}, &version)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return "", remote.FatalError{err}
	} else if err != nil && err.Error() == "rpc: can't find method RPCServer.Version" {
		// "Version" is not supported by this version of fluxd (it is old). Fail
		// gracefully.
		return "unknown", nil
	}
	return version, err
}

// Export is used to get service configuration in cluster-specific format
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

func (p *RPCClientV6) ListWorkloads(ctx context.Context, namespace string) ([]v6.WorkloadStatus, error) {
	var services []v6.WorkloadStatus
	err := p.client.Call("RPCServer.ListWorkloads", namespace, &services)
	listWorkloadsRolloutStatus(services)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return nil, remote.FatalError{err}
	}
	if err != nil {
		err = remoteApplicationError(err)
	}
	return services, err
}

func (p *RPCClientV6) ListWorkloadsWithOptions(ctx context.Context, opts v11.ListWorkloadsOptions) ([]v6.WorkloadStatus, error) {
	return listWorkloadsWithOptions(ctx, p, opts, supportedKindsV6)
}

func (p *RPCClientV6) ListImages(ctx context.Context, spec update.ResourceSpec) ([]v6.ImageStatus, error) {
	var images []v6.ImageStatus
	if err := requireWorkloadSpecKinds(spec, supportedKindsV6); err != nil {
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

func (p *RPCClientV6) ListImagesWithOptions(ctx context.Context, opts v10.ListImagesOptions) ([]v6.ImageStatus, error) {
	return listImagesWithOptions(ctx, p, opts)
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

func (p *RPCClientV6) GitRepoConfig(ctx context.Context, regenerate bool) (v6.GitConfig, error) {
	var result v6.GitConfig
	err := p.client.Call("RPCServer.GitRepoConfig", regenerate, &result)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return v6.GitConfig{}, remote.FatalError{err}
	}
	if err != nil {
		err = remoteApplicationError(err)
	}
	return result, err
}
