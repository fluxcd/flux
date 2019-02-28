package rpc

import (
	"context"
	"io"
	"net/rpc"
	"net/rpc/jsonrpc"

	"github.com/weaveworks/flux/api/v10"

	"github.com/pkg/errors"

	"github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/api/v6"
	"github.com/weaveworks/flux/api/v9"
	fluxerr "github.com/weaveworks/flux/errors"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/update"
)

// Server takes an api.Server and makes it available over RPC.
type Server struct {
	server *rpc.Server
}

// NewServer instantiates a new RPC server, handling requests on the
// conn by invoking methods on the underlying (assumed local) server.
func NewServer(s api.UpstreamServer) (*Server, error) {
	server := rpc.NewServer()
	if err := server.Register(&RPCServer{s}); err != nil {
		return nil, err
	}
	return &Server{server: server}, nil
}

func (c *Server) ServeConn(conn io.ReadWriteCloser) {
	c.server.ServeCodec(jsonrpc.NewServerCodec(conn))
}

type RPCServer struct {
	s api.UpstreamServer
}

func (p *RPCServer) Ping(_ struct{}, _ *struct{}) error {
	return p.s.Ping(context.Background())
}

func (p *RPCServer) Version(_ struct{}, resp *string) error {
	v, err := p.s.Version(context.Background())
	*resp = v
	return err
}

type ExportResponse struct {
	Result           []byte
	ApplicationError *fluxerr.Error
}

func (p *RPCServer) Export(_ struct{}, resp *ExportResponse) error {
	v, err := p.s.Export(context.Background())
	resp.Result = v
	if err != nil {
		if err, ok := errors.Cause(err).(*fluxerr.Error); ok {
			resp.ApplicationError = err
			return nil
		}
	}
	return err
}

type ListWorkloadsResponse struct {
	Result           []v6.WorkloadStatus
	ApplicationError *fluxerr.Error
}

func (p *RPCServer) ListWorkloads(namespace string, resp *ListWorkloadsResponse) error {
	v, err := p.s.ListWorkloads(context.Background(), namespace)
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
	Result           []v6.ImageStatus
	ApplicationError *fluxerr.Error
}

func (p *RPCServer) ListImages(spec update.ResourceSpec, resp *ListImagesResponse) error {
	v, err := p.s.ListImages(context.Background(), spec)
	resp.Result = v
	if err != nil {
		if err, ok := errors.Cause(err).(*fluxerr.Error); ok {
			resp.ApplicationError = err
			return nil
		}
	}
	return err
}

func (p *RPCServer) ListImagesWithOptions(opts v10.ListImagesOptions, resp *ListImagesResponse) error {
	v, err := p.s.ListImagesWithOptions(context.Background(), opts)
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
	v, err := p.s.UpdateManifests(context.Background(), spec)
	resp.Result = v
	if err != nil {
		if err, ok := errors.Cause(err).(*fluxerr.Error); ok {
			resp.ApplicationError = err
			return nil
		}
	}
	return err
}

type NotifyChangeResponse struct {
	ApplicationError *fluxerr.Error
}

func (p *RPCServer) NotifyChange(c v9.Change, resp *NotifyChangeResponse) error {
	err := p.s.NotifyChange(context.Background(), c)
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
	v, err := p.s.JobStatus(context.Background(), jobID)
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
	v, err := p.s.SyncStatus(context.Background(), ref)
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
	Result           v6.GitConfig
	ApplicationError *fluxerr.Error
}

func (p *RPCServer) GitRepoConfig(regenerate bool, resp *GitRepoConfigResponse) error {
	v, err := p.s.GitRepoConfig(context.Background(), regenerate)
	resp.Result = v
	if err != nil {
		if err, ok := errors.Cause(err).(*fluxerr.Error); ok {
			resp.ApplicationError = err
			return nil
		}
	}
	return err
}
