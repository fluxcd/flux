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
type ApplyResult map[flux.ServiceID]string

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
	return p.p.Ping()
}

func (p *RPCServer) Version(_ struct{}, resp *string) error {
	v, err := p.p.Version()
	*resp = v
	return err
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

// Regrade is still around for backwards compatibility, though it is called "Apply" everywhere else.
func (p *RPCServer) Regrade(defs []platform.ServiceDefinition, applyResult *ApplyResult) error {
	return p.Apply(defs, applyResult)
}

func (p *RPCServer) Apply(defs []platform.ServiceDefinition, applyResult *ApplyResult) error {
	result := ApplyResult{}
	err := p.p.Apply(defs)
	if err != nil {
		switch applyErr := err.(type) {
		case platform.ApplyError:
			for s, e := range applyErr {
				result[s] = e.Error()
			}
			err = nil
		}
	}
	*applyResult = result
	return err
}
