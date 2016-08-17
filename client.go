package flux

import (
	"net/http"

	"github.com/gorilla/mux"
)

type client struct {
	client   *http.Client
	router   *mux.Router
	endpoint string
}

func NewClient(c *http.Client, router *mux.Router, endpoint string) Service {
	return &client{
		client:   c,
		router:   router,
		endpoint: endpoint,
	}
}

func (c *client) ListServices() ([]ServiceStatus, error) {
	return invokeListServices(c.client, c.router, c.endpoint)
}

func (c *client) ListImages(s ServiceSpec) ([]ImageStatus, error) {
	return invokeListImages(c.client, c.router, c.endpoint, s)
}

func (c *client) Release(s ServiceSpec, i ImageSpec, k ReleaseKind) ([]ReleaseAction, error) {
	return invokeRelease(c.client, c.router, c.endpoint, s, i, k)
}

func (c *client) Automate(id ServiceID) error {
	return invokeAutomate(c.client, c.router, c.endpoint, id)
}

func (c *client) Deautomate(id ServiceID) error {
	return invokeDeautomate(c.client, c.router, c.endpoint, id)
}

func (c *client) History(s ServiceSpec) ([]HistoryEntry, error) {
	return invokeHistory(c.client, c.router, c.endpoint, s)
}
