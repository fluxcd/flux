package http

import (
	"net/http"
	"net/url"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	"github.com/weaveworks/fluxy"
	"github.com/weaveworks/fluxy/http/websocket"
	"github.com/weaveworks/fluxy/platform"
	"github.com/weaveworks/fluxy/platform/rpc"
)

// Daemon handles communication from the daemon to the service
type Daemon struct {
	client   *http.Client
	token    flux.Token
	url      *url.URL
	platform platform.Platform
	logger   log.Logger
	quit     chan struct{}

	ws websocket.Websocket
}

func NewDaemon(client *http.Client, t flux.Token, router *mux.Router, endpoint string, p platform.Platform, logger log.Logger) (*Daemon, error) {
	u, err := makeURL(endpoint, router, "Daemon")
	if err != nil {
		return nil, errors.Wrap(err, "constructing URL")
	}

	a := &Daemon{
		client:   client,
		token:    t,
		url:      u,
		platform: p,
		logger:   logger,
		quit:     make(chan struct{}),
	}
	go a.loop()
	return a, nil
}

func (a *Daemon) loop() {
	backoff := 5 * time.Second
	errc := make(chan error)
	for {
		go func() {
			errc <- a.connect()
		}()
		select {
		case err := <-errc:
			if err != nil {
				a.logger.Log("err", err)
				time.Sleep(backoff)
				continue
			}
		case <-a.quit:
			return
		}
	}
}

func (a *Daemon) connect() error {
	a.logger.Log("connecting", true)
	ws, err := websocket.Dial(a.client, a.token, a.url)
	if err != nil {
		return errors.Wrapf(err, "executing websocket %s", a.url)
	}
	a.ws = ws
	defer func() {
		a.ws = nil
		// TODO: handle this error
		a.logger.Log("connection closing", true, "err", ws.Close())
	}()
	a.logger.Log("connected", true)

	// Hook up the rpc client
	client, err := rpc.NewClient(a.platform)
	if err != nil {
		return errors.Wrap(err, "initializing rpc client")
	}
	client.ServeConn(ws)
	a.logger.Log("disconnected", true)
	return nil
}

// Close closes the connection to the service
func (a *Daemon) Close() error {
	close(a.quit)
	if a.ws == nil {
		return nil
	}
	return a.ws.Close()
}
