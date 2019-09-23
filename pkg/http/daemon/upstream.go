package daemon

// This file can be removed from the package once `--connect` is
// removed from fluxd. Until then, it will be imported from here.

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics/prometheus"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	stdprometheus "github.com/prometheus/client_golang/prometheus"

	"github.com/fluxcd/flux/pkg/api"
	"github.com/fluxcd/flux/pkg/event"
	transport "github.com/fluxcd/flux/pkg/http"
	fluxclient "github.com/fluxcd/flux/pkg/http/client"
	"github.com/fluxcd/flux/pkg/http/websocket"
	"github.com/fluxcd/flux/pkg/remote/rpc"
)

// Upstream handles communication from the daemon to a service
type Upstream struct {
	client    *http.Client
	ua        string
	token     fluxclient.Token
	url       *url.URL
	endpoint  string
	apiClient *fluxclient.Client
	server    api.Server
	timeout   time.Duration
	logger    log.Logger
	quit      chan struct{}

	ws websocket.Websocket
}

var (
	ErrEndpointDeprecated = errors.New("Your fluxd version is deprecated - please upgrade, see https://github.com/weaveworks/flux-adapter")
	connectionDuration    = prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
		Namespace: "flux",
		Subsystem: "fluxd",
		Name:      "connection_duration_seconds",
		Help:      "Duration in seconds of the current connection to fluxsvc. Zero means unconnected.",
	}, []string{"target"})
)

func NewUpstream(client *http.Client, ua string, t fluxclient.Token, router *mux.Router, endpoint string, s api.Server, timeout time.Duration, logger log.Logger) (*Upstream, error) {
	httpEndpoint, wsEndpoint, err := inferEndpoints(endpoint)
	if err != nil {
		return nil, errors.Wrap(err, "inferring WS/HTTP endpoints")
	}

	u, err := transport.MakeURL(wsEndpoint, router, transport.RegisterDaemonV11)
	if err != nil {
		return nil, errors.Wrap(err, "constructing URL")
	}

	a := &Upstream{
		client:    client,
		ua:        ua,
		token:     t,
		url:       u,
		endpoint:  wsEndpoint,
		apiClient: fluxclient.New(client, router, httpEndpoint, t),
		server:    s,
		timeout:   timeout,
		logger:    logger,
		quit:      make(chan struct{}),
	}
	go a.loop()
	return a, nil
}

func inferEndpoints(endpoint string) (httpEndpoint, wsEndpoint string, err error) {
	endpointURL, err := url.Parse(endpoint)
	if err != nil {
		return "", "", errors.Wrapf(err, "parsing endpoint %s", endpoint)
	}

	switch endpointURL.Scheme {
	case "ws":
		httpURL := *endpointURL
		httpURL.Scheme = "http"
		return httpURL.String(), endpointURL.String(), nil
	case "wss":
		httpURL := *endpointURL
		httpURL.Scheme = "https"
		return httpURL.String(), endpointURL.String(), nil
	case "http":
		wsURL := *endpointURL
		wsURL.Scheme = "ws"
		return endpointURL.String(), wsURL.String(), nil
	case "https":
		wsURL := *endpointURL
		wsURL.Scheme = "wss"
		return endpointURL.String(), wsURL.String(), nil
	default:
		return "", "", errors.Errorf("unsupported scheme %s", endpointURL.Scheme)
	}
}

func (a *Upstream) loop() {
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
				if err == ErrEndpointDeprecated {
					// We have logged the deprecation error, now crashloop to garner attention
					os.Exit(1)
				}
			}
			time.Sleep(backoff)
		case <-a.quit:
			return
		}
	}
}

func (a *Upstream) connect() error {
	a.setConnectionDuration(0)
	a.logger.Log("connecting", true)
	ws, err := websocket.Dial(a.client, a.ua, a.token, a.url)
	if err != nil {
		if err, ok := err.(*websocket.DialErr); ok && err.HTTPResponse != nil && err.HTTPResponse.StatusCode == http.StatusGone {
			return ErrEndpointDeprecated
		}
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
	rpcserver, err := rpc.NewServer(a.server, a.timeout)
	if err != nil {
		return errors.Wrap(err, "initializing rpc server")
	}
	rpcserver.ServeConn(ws)
	a.logger.Log("disconnected", true)
	return nil
}

func (a *Upstream) setConnectionDuration(duration float64) {
	connectionDuration.With("target", a.endpoint).Set(duration)
}

func (a *Upstream) LogEvent(event event.Event) error {
	return a.apiClient.LogEvent(context.TODO(), event)
}

// Close closes the connection to the service
func (a *Upstream) Close() error {
	close(a.quit)
	if a.ws == nil {
		return nil
	}
	return a.ws.Close()
}
