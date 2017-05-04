package rpc

import (
	"io"
	"net/rpc"
	"net/rpc/jsonrpc"

	"github.com/weaveworks/flux"
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
	return p.p.Ping()
}

func (p *RPCServer) Version(_ struct{}, resp *string) error {
	v, err := p.p.Version()
	*resp = v
	return err
}

func (p *RPCServer) Export(_ struct{}, resp *[]byte) error {
	v, err := p.p.Export()
	*resp = v
	return err
}

func (p *RPCServer) ListServices(namespace string, resp *[]flux.ServiceStatus) error {
	v, err := p.p.ListServices(namespace)
	*resp = v
	return err
}

func (p *RPCServer) ListImages(spec update.ServiceSpec, resp *[]flux.ImageStatus) error {
	v, err := p.p.ListImages(spec)
	*resp = v
	return err
}

func (p *RPCServer) UpdateManifests(spec update.Spec, resp *job.ID) error {
	v, err := p.p.UpdateManifests(spec)
	*resp = v
	return err
}

func (p *RPCServer) SyncNotify(_ struct{}, _ *struct{}) error {
	return p.p.SyncNotify()
}

func (p *RPCServer) JobStatus(jobID job.ID, resp *job.Status) error {
	v, err := p.p.JobStatus(jobID)
	*resp = v
	return err
}

func (p *RPCServer) SyncStatus(cursor string, resp *[]string) error {
	v, err := p.p.SyncStatus(cursor)
	*resp = v
	return err
}
