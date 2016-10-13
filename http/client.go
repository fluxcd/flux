package http

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/weaveworks/fluxy"
	clientAPI "github.com/weaveworks/fluxy/client"
)

type client struct {
	client   *http.Client
	token    flux.Token
	router   *mux.Router
	endpoint string
}

func NewClient(c *http.Client, router *mux.Router, endpoint string, t flux.Token) clientAPI.Client {
	return &client{
		client:   c,
		token:    t,
		router:   router,
		endpoint: endpoint,
	}
}

func (c *client) ListServices(_ flux.InstanceID, namespace string) ([]flux.ServiceStatus, error) {
	return invokeListServices(c.client, c.token, c.router, c.endpoint, namespace)
}

func (c *client) ListImages(_ flux.InstanceID, s flux.ServiceSpec) ([]flux.ImageStatus, error) {
	return invokeListImages(c.client, c.token, c.router, c.endpoint, s)
}

func (c *client) PostRelease(_ flux.InstanceID, s flux.ReleaseJobSpec) (flux.ReleaseID, error) {
	return invokePostRelease(c.client, c.token, c.router, c.endpoint, s)
}

func (c *client) GetRelease(_ flux.InstanceID, id flux.ReleaseID) (flux.ReleaseJob, error) {
	return invokeGetRelease(c.client, c.token, c.router, c.endpoint, id)
}

func (c *client) Automate(_ flux.InstanceID, id flux.ServiceID) error {
	return invokeAutomate(c.client, c.token, c.router, c.endpoint, id)
}

func (c *client) Deautomate(_ flux.InstanceID, id flux.ServiceID) error {
	return invokeDeautomate(c.client, c.token, c.router, c.endpoint, id)
}

func (c *client) Lock(_ flux.InstanceID, id flux.ServiceID) error {
	return invokeLock(c.client, c.token, c.router, c.endpoint, id)
}

func (c *client) Unlock(_ flux.InstanceID, id flux.ServiceID) error {
	return invokeUnlock(c.client, c.token, c.router, c.endpoint, id)
}

func (c *client) History(_ flux.InstanceID, s flux.ServiceSpec) ([]flux.HistoryEntry, error) {
	return invokeHistory(c.client, c.token, c.router, c.endpoint, s)
}

func (c *client) GetConfig(_ flux.InstanceID, secrets bool) (flux.InstanceConfig, error) {
	return invokeGetConfig(c.client, c.token, c.router, c.endpoint, secrets)
}

func (c *client) SetConfig(_ flux.InstanceID, config flux.InstanceConfig) error {
	return invokeSetConfig(c.client, c.token, c.router, c.endpoint, config)
}
