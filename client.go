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

func (c *client) ListServices(namespace string) ([]ServiceStatus, error) {
	return invokeListServices(c.client, c.router, c.endpoint, namespace)
}

func (c *client) ListImages(s ServiceSpec) ([]ImageStatus, error) {
	return invokeListImages(c.client, c.router, c.endpoint, s)
}

func (c *client) PostRelease(s ReleaseJobSpec) (ReleaseID, error) {
	return invokePostRelease(c.client, c.router, c.endpoint, s)
}

func (c *client) GetRelease(id ReleaseID) (ReleaseJob, error) {
	return invokeGetRelease(c.client, c.router, c.endpoint, id)
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
