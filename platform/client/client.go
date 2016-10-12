package client

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/rpc"
	"net/url"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/websocket"

	"github.com/weaveworks/fluxy/platform"
)

type Client struct {
	addr   url.URL
	token  string
	p      platform.Platform
	logger log.Logger

	wsDialer websocket.Dialer
	quit     chan struct{}
}

func New(addr, token string, insecure bool, p platform.Platform, logger log.Logger) (*Client, error) {
	target, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}
	c := &Client{
		addr:   *target,
		token:  token,
		p:      p,
		logger: logger,

		wsDialer: websocket.Dialer{
			TLSClientConfig: &tls.Config{ServerName: target.Host, InsecureSkipVerify: insecure},
		},
		quit: make(chan struct{}),
	}
	go c.loop()
	return c, nil
}

func (c *Client) connect() (Websocket, error) {
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Scope-Probe token=%s", c.token))
	url := c.wsURL("/api/fluxy/ws")
	conn, _, err := DialWS(&c.wsDialer, url, headers)
	return conn, err
}

func (c *Client) wsURL(path string) string {
	output := c.addr //copy the url
	if output.Scheme == "https" {
		output.Scheme = "wss"
	} else {
		output.Scheme = "ws"
	}
	return output.String() + path
}

func (c *Client) rpcServer() (*rpc.Server, error) {
	server := rpc.NewServer()
	for name, handler := range map[string]interface{}{
		"AllServices":  c.p,
		"SomeServices": c.p,
		"Regrade":      c.p,
	} {
		if err := server.RegisterName(name, handler); err != nil {
			return nil, err
		}
	}
	return server, nil
}

func (c *Client) loop() {
	backoff := 5 * time.Second
	errc := make(chan error)
	for {
		go func() {
			server, err := c.rpcServer()
			if err != nil {
				errc <- err
				return
			}
			conn, err := c.connect()
			if err != nil {
				errc <- err
				return
			}
			server.ServeCodec(NewJSONWebsocketCodec(conn))
			conn.Close()
		}()
		select {
		case err := <-errc:
			if err != nil {
				c.logger.Log("err", err)
				time.Sleep(backoff)
				continue
			}
		case <-c.quit:
			return
		}
	}
}

func (c *Client) Close() error {
	close(c.quit)
	return nil
}
