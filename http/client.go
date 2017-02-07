package http

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/jobs"
)

type client struct {
	client   *http.Client
	token    flux.Token
	router   *mux.Router
	endpoint string
}

func NewClient(c *http.Client, router *mux.Router, endpoint string, t flux.Token) api.ClientService {
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

func (c *client) PostRelease(_ flux.InstanceID, s jobs.ReleaseJobParams) (jobs.JobID, error) {
	return invokePostRelease(c.client, c.token, c.router, c.endpoint, s)
}

func (c *client) GetRelease(_ flux.InstanceID, id jobs.JobID) (jobs.Job, error) {
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

func (c *client) GetConfig(_ flux.InstanceID) (flux.InstanceConfig, error) {
	return invokeGetConfig(c.client, c.token, c.router, c.endpoint)
}

func (c *client) SetConfig(_ flux.InstanceID, config flux.UnsafeInstanceConfig) error {
	return invokeSetConfig(c.client, c.token, c.router, c.endpoint, config)
}

func (c *client) GenerateDeployKey(_ flux.InstanceID) error {
	return invokeGenerateKeys(c.client, c.token, c.router, c.endpoint)
}

func (c *client) Status(_ flux.InstanceID) (flux.Status, error) {
	return invokeStatus(c.client, c.token, c.router, c.endpoint)
}
