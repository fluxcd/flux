package rpc

import (
	"io"
	"net/rpc"
	"net/rpc/jsonrpc"

	"github.com/weaveworks/fluxy"
	"github.com/weaveworks/fluxy/platform"
)

// Client takes a platform and makes it available as an RPC
type Client struct {
	server *rpc.Server
}

// NewClient instantiates a new RPC client, handling requests on the conn
func NewClient(p platform.Platform) (*Client, error) {
	server := rpc.NewServer()
	if err := server.Register(&RPCClientPlatform{p}); err != nil {
		return nil, err
	}
	return &Client{server: server}, nil
}

func (c *Client) ServeConn(conn io.ReadWriteCloser) {
	c.server.ServeCodec(jsonrpc.NewServerCodec(conn))
}

type RPCClientPlatform struct {
	p platform.Platform
}

func (p *RPCClientPlatform) Ping(_ struct{}, _ *struct{}) error {
	return nil
}

func (p *RPCClientPlatform) AllServices(req AllServicesRequest, resp *[]platform.Service) error {
	s, err := p.p.AllServices(req.MaybeNamespace, req.Ignored)
	*resp = s
	return err
}

func (p *RPCClientPlatform) SomeServices(ids []flux.ServiceID, resp *[]platform.Service) error {
	s, err := p.p.SomeServices(ids)
	*resp = s
	return err
}

func (p *RPCClientPlatform) Regrade(spec []platform.RegradeSpec, _ *struct{}) error {
	return p.p.Regrade(spec)
}
