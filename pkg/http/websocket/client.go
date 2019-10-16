package websocket

import (
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"

	"github.com/fluxcd/flux/pkg/http/client"
)

type DialErr struct {
	URL          *url.URL
	HTTPResponse *http.Response
}

func (de DialErr) Error() string {
	if de.URL != nil && de.HTTPResponse != nil {
		return fmt.Sprintf("connecting to websocket %s (http status code = %v)", de.URL, de.HTTPResponse.StatusCode)
	}
	return "connecting to websocket (unknown error)"
}

// Dial initiates a new websocket connection.
func Dial(client *http.Client, ua string, token client.Token, u *url.URL) (Websocket, error) {
	// Build the http request
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, errors.Wrapf(err, "constructing request %s", u)
	}

	// Send version in user-agent
	req.Header.Set("User-Agent", ua)

	// Add authentication if provided
	token.Set(req)

	// Use http client to do the http request
	conn, resp, err := dialer(client).Dial(u.String(), req.Header)
	if err != nil {
		if resp != nil {
			err = &DialErr{u, resp}
		}
		return nil, err
	}

	// Set up the ping heartbeat
	return Ping(conn), nil
}

func dialer(client *http.Client) *websocket.Dialer {
	return &websocket.Dialer{
		NetDial: func(network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, client.Timeout)
		},
		HandshakeTimeout: client.Timeout,
		Jar:              client.Jar,
		// TODO: TLSClientConfig: client.TLSClientConfig,
		// TODO: Proxy
	}
}
