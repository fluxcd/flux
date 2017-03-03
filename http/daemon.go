package http

import (
	"net/http"
	"net/url"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/http/websocket"
	"github.com/weaveworks/flux/platform"
	"github.com/weaveworks/flux/platform/rpc"
)

// Daemon handles communication from the daemon to the service
type Daemon struct {
	client   *http.Client
	token    flux.Token
	url      *url.URL
	endpoint string
	platform platform.Platform
	logger   log.Logger
	quit     chan struct{}

	ws websocket.Websocket
}

var (
	connectionDuration = prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
		Namespace: "flux",
		Subsystem: "fluxd",
		Name:      "connection_duration_seconds",
		Help:      "Duration in seconds of the current connection to fluxsvc. Zero means unconnected.",
	}, []string{"target"})
)

func NewDaemon(client *http.Client, t flux.Token, router *mux.Router, endpoint string, p platform.Platform, logger log.Logger) (*Daemon, error) {
	u, err := MakeURL(endpoint, router, "RegisterDaemon")
	if err != nil {
		return nil, errors.Wrap(err, "constructing URL")
	}

	a := &Daemon{
		client:   client,
		token:    t,
		url:      u,
		endpoint: endpoint,
		platform: p,
		logger:   logger,
		quit:     make(chan struct{}),
	}
	go a.loop()
	return a, nil
}

func (a *Daemon) loop() {
	backoff := 5 * time.Second
	errc := make(chan error, 1)
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
	a.setConnectionDuration(0)
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

	// Instrument connection lifespan
	connectedAt := time.Now()
	disconnected := make(chan struct{})
	defer close(disconnected)
	go func() {
		t := time.NewTicker(1 * time.Second)
		for {
			select {
			case now := <-t.C:
				a.setConnectionDuration(now.Sub(connectedAt).Seconds())
			case <-disconnected:
				t.Stop()
				a.setConnectionDuration(0)
				return
			}
		}
	}()

	// Hook up the rpc server. We are a websocket _client_, but an RPC
	// _server_.
	rpcserver, err := rpc.NewServer(a.platform)
	if err != nil {
		return errors.Wrap(err, "initializing rpc client")
	}
	rpcserver.ServeConn(ws)
	a.logger.Log("disconnected", true)
	return nil
}

func (a *Daemon) setConnectionDuration(duration float64) {
	connectionDuration.With("target", a.endpoint).Set(duration)
}

// Close closes the connection to the service
func (a *Daemon) Close() error {
	close(a.quit)
	if a.ws == nil {
		return nil
	}
	return a.ws.Close()
}
