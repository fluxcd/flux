package http

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/weaveworks/fluxy"
)

type client struct {
	client   *http.Client
	router   *mux.Router
	endpoint string
}

func NewClient(c *http.Client, router *mux.Router, endpoint string) flux.Service {
	return &client{
		client:   c,
		router:   router,
		endpoint: endpoint,
	}
}

func (c *client) ListServices(namespace string) ([]flux.ServiceStatus, error) {
	return invokeListServices(c.client, c.router, c.endpoint, namespace)
}

func (c *client) ListImages(s flux.ServiceSpec) ([]flux.ImageStatus, error) {
	return invokeListImages(c.client, c.router, c.endpoint, s)
}

func (c *client) PostRelease(s flux.ReleaseJobSpec) (flux.ReleaseID, error) {
	return invokePostRelease(c.client, c.router, c.endpoint, s)
}

func (c *client) GetRelease(id flux.ReleaseID) (flux.ReleaseJob, error) {
	return invokeGetRelease(c.client, c.router, c.endpoint, id)
}

func (c *client) Automate(id flux.ServiceID) error {
	return invokeAutomate(c.client, c.router, c.endpoint, id)
}

func (c *client) Deautomate(id flux.ServiceID) error {
	return invokeDeautomate(c.client, c.router, c.endpoint, id)
}

func (c *client) History(s flux.ServiceSpec) ([]flux.HistoryEntry, error) {
	return invokeHistory(c.client, c.router, c.endpoint, s)
}
