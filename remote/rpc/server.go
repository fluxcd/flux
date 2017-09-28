package rpc

import (
	"context"
	"io"
	"net/rpc"
	"net/rpc/jsonrpc"

	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	fluxerr "github.com/weaveworks/flux/errors"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/update"
)

// Server takes a platform and makes it available over RPC.
type Server struct {
	server *rpc.Server
}

// NewServer instantiates a new RPC server, handling requests on the
// conn by invoking methods on the underlying (assumed local)
// platform.
func NewServer(p remote.Platform) (*Server, error) {
	server := rpc.NewServer()
	if err := server.Register(&RPCServer{p}); err != nil {
		return nil, err
	}
	return &Server{server: server}, nil
}

func (c *Server) ServeConn(conn io.ReadWriteCloser) {
	c.server.ServeCodec(jsonrpc.NewServerCodec(conn))
}

type RPCServer struct {
	p remote.Platform
}

func (p *RPCServer) Ping(_ struct{}, _ *struct{}) error {
	return p.p.Ping(context.Background())
}

func (p *RPCServer) Version(_ struct{}, resp *string) error {
	v, err := p.p.Version(context.Background())
	*resp = v
	return err
}

type ExportResponse struct {
	Result           []byte
	ApplicationError *fluxerr.Error
}

func (p *RPCServer) Export(_ struct{}, resp *ExportResponse) error {
	v, err := p.p.Export(context.Background())
	resp.Result = v
	if err != nil {
		if err, ok := errors.Cause(err).(*fluxerr.Error); ok {
			resp.ApplicationError = err
			return nil
		}
	}
	return err
}

type ListServicesResponse struct {
	Result           []flux.ServiceStatus
	ApplicationError *fluxerr.Error
}

func (p *RPCServer) ListServices(namespace string, resp *ListServicesResponse) error {
	v, err := p.p.ListServices(context.Background(), namespace)
	resp.Result = v
	if err != nil {
		if err, ok := errors.Cause(err).(*fluxerr.Error); ok {
			resp.ApplicationError = err
			return nil
		}
	}
	return err
}

type ListImagesResponse struct {
	Result           []flux.ImageStatus
	ApplicationError *fluxerr.Error
}

func (p *RPCServer) ListImages(spec update.ServiceSpec, resp *ListImagesResponse) error {
	v, err := p.p.ListImages(context.Background(), spec)
	resp.Result = v
	if err != nil {
		if err, ok := errors.Cause(err).(*fluxerr.Error); ok {
			resp.ApplicationError = err
			return nil
		}
	}
	return err
}

type UpdateManifestsResponse struct {
	Result           job.ID
	ApplicationError *fluxerr.Error
}

func (p *RPCServer) UpdateManifests(spec update.Spec, resp *UpdateManifestsResponse) error {
	v, err := p.p.UpdateManifests(context.Background(), spec)
	resp.Result = v
	if err != nil {
		if err, ok := errors.Cause(err).(*fluxerr.Error); ok {
			resp.ApplicationError = err
			return nil
		}
	}
	return err
}

type SyncNotifyResponse struct {
	ApplicationError *fluxerr.Error
}

func (p *RPCServer) SyncNotify(_ struct{}, resp *SyncNotifyResponse) error {
	err := p.p.SyncNotify(context.Background())
	if err != nil {
		if err, ok := errors.Cause(err).(*fluxerr.Error); ok {
			resp.ApplicationError = err
			return nil
		}
	}
	return err
}

type JobStatusResponse struct {
	Result           job.Status
	ApplicationError *fluxerr.Error
}

func (p *RPCServer) JobStatus(jobID job.ID, resp *JobStatusResponse) error {
	v, err := p.p.JobStatus(context.Background(), jobID)
	resp.Result = v
	if err != nil {
		if err, ok := errors.Cause(err).(*fluxerr.Error); ok {
			resp.ApplicationError = err
			return nil
		}
	}
	return err
}

type SyncStatusResponse struct {
	Result           []string
	ApplicationError *fluxerr.Error
}

func (p *RPCServer) SyncStatus(ref string, resp *SyncStatusResponse) error {
	v, err := p.p.SyncStatus(context.Background(), ref)
	resp.Result = v
	if err != nil {
		if err, ok := errors.Cause(err).(*fluxerr.Error); ok {
			resp.ApplicationError = err
			return nil
		}
	}
	return err
}

type GitRepoConfigResponse struct {
	Result           flux.GitConfig
	ApplicationError *fluxerr.Error
}

func (p *RPCServer) GitRepoConfig(regenerate bool, resp *GitRepoConfigResponse) error {
	v, err := p.p.GitRepoConfig(context.Background(), regenerate)
	resp.Result = v
	if err != nil {
		if err, ok := errors.Cause(err).(*fluxerr.Error); ok {
			resp.ApplicationError = err
			return nil
		}
	}
	return err
}
