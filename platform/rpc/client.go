package rpc

import (
	"io"
	"net/rpc"
	"net/rpc/jsonrpc"

	"github.com/weaveworks/fluxy"
	"github.com/weaveworks/fluxy/platform"
)

// net/rpc cannot serialise errors, so we transmit strings and
// reconstitute them on the other side.
type RegradeResult map[flux.ServiceID]string

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
	if s == nil {
		s = []platform.Service{}
	}
	*resp = s
	return err
}

func (p *RPCClientPlatform) SomeServices(ids []flux.ServiceID, resp *[]platform.Service) error {
	s, err := p.p.SomeServices(ids)
	if s == nil {
		s = []platform.Service{}
	}
	*resp = s
	return err
}

func (p *RPCClientPlatform) Regrade(spec []platform.RegradeSpec, regradeError *RegradeResult) error {
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
