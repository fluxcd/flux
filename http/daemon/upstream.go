package daemon

import (
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics/prometheus"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	stdprometheus "github.com/prometheus/client_golang/prometheus"

	"github.com/weaveworks/flux"
	transport "github.com/weaveworks/flux/http"
	fluxclient "github.com/weaveworks/flux/http/client"
	"github.com/weaveworks/flux/http/websocket"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/remote/rpc"
)

// Upstream handles communication from the daemon to a service
type Upstream struct {
	client    *http.Client
	ua        string
	token     flux.Token
	url       *url.URL
	endpoint  string
	apiClient *fluxclient.Client
	platform  remote.Platform
	logger    log.Logger
	quit      chan struct{}

	ws websocket.Websocket
}

var (
	ErrEndpointDeprecated = errors.New("Your fluxd version is deprecated - please upgrade, see https://github.com/weaveworks/flux/releases")
	connectionDuration    = prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
		Namespace: "flux",
		Subsystem: "fluxd",
		Name:      "connection_duration_seconds",
		Help:      "Duration in seconds of the current connection to fluxsvc. Zero means unconnected.",
	}, []string{"target"})
	urlSchemeRE = regexp.MustCompile("^([[:alpha:]]+)(s?)://")
)

func NewUpstream(client *http.Client, ua string, t flux.Token, router *mux.Router, endpoint string, p remote.Platform, logger log.Logger) (*Upstream, error) {
	u, err := transport.MakeURL(endpoint, router, "RegisterDaemonV6")
	if err != nil {
		return nil, errors.Wrap(err, "constructing URL")
	}

	// TODO: hacky regex hacks are hacky
	httpEndpoint := urlSchemeRE.ReplaceAllString(endpoint, "http${2}://")

	a := &Upstream{
		client:   client,
		ua:       ua,
		token:    t,
		url:      u,
		endpoint: endpoint,
		// FIXME This endpoint might be a wss, and we might need to swap it for an https...
		apiClient: fluxclient.New(client, router, httpEndpoint, t),
		platform:  p,
		logger:    logger,
		quit:      make(chan struct{}),
	}
	go a.loop()
	return a, nil
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
				time.Sleep(backoff)
				continue
			}
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
	rpcserver, err := rpc.NewServer(a.platform)
	if err != nil {
		return errors.Wrap(err, "initializing rpc client")
	}
	rpcserver.ServeConn(ws)
	a.logger.Log("disconnected", true)
	return nil
}

func (a *Upstream) setConnectionDuration(duration float64) {
	connectionDuration.With("target", a.endpoint).Set(duration)
}

func (a *Upstream) LogEvent(event flux.Event) error {
	// Instance ID is set via token here, so we can leave it blank.
	return a.apiClient.LogEvent(flux.InstanceID(""), event)
}

// Close closes the connection to the service
func (a *Upstream) Close() error {
	close(a.quit)
	if a.ws == nil {
		return nil
	}
	return a.ws.Close()
}
