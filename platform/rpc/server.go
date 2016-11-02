package rpc

import (
	"io"
	"net/rpc"
	"net/rpc/jsonrpc"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/platform"
)

// net/rpc cannot serialise errors, so we transmit strings and
// reconstitute them on the other side.
type RegradeResult map[flux.ServiceID]string

// Server takes a platform and makes it available over RPC.
type Server struct {
	server *rpc.Server
}

// NewServer instantiates a new RPC server, handling requests on the
// conn by invoking methods on the underlying (assumed local)
// platform.
func NewServer(p platform.Platform) (*Server, error) {
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
	p platform.Platform
}

func (p *RPCServer) Ping(_ struct{}, _ *struct{}) error {
	return nil
}

func (p *RPCServer) AllServices(req AllServicesRequest, resp *[]platform.Service) error {
	s, err := p.p.AllServices(req.MaybeNamespace, req.Ignored)
	if s == nil {
		s = []platform.Service{}
	}
	*resp = s
	return err
}

func (p *RPCServer) SomeServices(ids []flux.ServiceID, resp *[]platform.Service) error {
	s, err := p.p.SomeServices(ids)
	if s == nil {
		s = []platform.Service{}
	}
	*resp = s
	return err
}

func (p *RPCServer) Regrade(spec []platform.RegradeSpec, regradeError *RegradeResult) error {
	result := RegradeResult{}
	err := p.p.Regrade(spec)
	if err != nil {
		switch err := err.(type) {
		case platform.RegradeError:
			for s, e := range err {
				result[s] = e.Error()
			}
		}
	}
	*regradeError = result
	return err
}
