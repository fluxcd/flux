package rpc

import (
	"context"
	"io"
	"net/rpc"
	"net/rpc/jsonrpc"
	"time"

	"github.com/fluxcd/flux/pkg/api/v10"

	"github.com/pkg/errors"

	"github.com/fluxcd/flux/pkg/api"
	"github.com/fluxcd/flux/pkg/api/v6"
	"github.com/fluxcd/flux/pkg/api/v9"
	fluxerr "github.com/fluxcd/flux/pkg/errors"
	"github.com/fluxcd/flux/pkg/job"
	"github.com/fluxcd/flux/pkg/update"
)

// Server takes an api.Server and makes it available over RPC.
type Server struct {
	server *rpc.Server
}

// NewServer instantiates a new RPC server, handling requests on the
// conn by invoking methods on the underlying (assumed local) server.
func NewServer(s api.Server, t time.Duration) (*Server, error) {
	server := rpc.NewServer()
	if err := server.Register(&RPCServer{s, t}); err != nil {
		return nil, err
	}
	return &Server{server}, nil
}

func (c *Server) ServeConn(conn io.ReadWriteCloser) {
	c.server.ServeCodec(jsonrpc.NewServerCodec(conn))
}

type RPCServer struct {
	s       api.Server
	timeout time.Duration
}

func (p *RPCServer) Ping(_ struct{}, _ *struct{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	return p.s.Ping(ctx)
}

func (p *RPCServer) Version(_ struct{}, resp *string) error {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	v, err := p.s.Version(ctx)
	*resp = v
	return err
}

type ExportResponse struct {
	Result           []byte
	ApplicationError *fluxerr.Error
}

func (p *RPCServer) Export(_ struct{}, resp *ExportResponse) error {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	v, err := p.s.Export(ctx)
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
	Result           []v6.ControllerStatus
	ApplicationError *fluxerr.Error
}

func (p *RPCServer) ListServices(namespace string, resp *ListServicesResponse) error {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	v, err := p.s.ListServices(ctx, namespace)
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
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	v, err := p.s.ListImages(ctx, spec)
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
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	v, err := p.s.ListImagesWithOptions(ctx, opts)
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
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	v, err := p.s.UpdateManifests(ctx, spec)
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
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	err := p.s.NotifyChange(ctx, c)
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
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	v, err := p.s.JobStatus(ctx, jobID)
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
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	v, err := p.s.SyncStatus(ctx, ref)
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
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	v, err := p.s.GitRepoConfig(ctx, regenerate)
	resp.Result = v
	if err != nil {
		if err, ok := errors.Cause(err).(*fluxerr.Error); ok {
			resp.ApplicationError = err
			return nil
		}
	}
	return err
}
