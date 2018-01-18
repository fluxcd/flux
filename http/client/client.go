package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	fluxerr "github.com/weaveworks/flux/errors"
	"github.com/weaveworks/flux/event"
	transport "github.com/weaveworks/flux/http"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/update"
)

type Client struct {
	client   *http.Client
	token    flux.Token
	router   *mux.Router
	endpoint string
}

func New(c *http.Client, router *mux.Router, endpoint string, t flux.Token) *Client {
	return &Client{
		client:   c,
		token:    t,
		router:   router,
		endpoint: endpoint,
	}
}

func (c *Client) ListServices(ctx context.Context, namespace string) ([]flux.ControllerStatus, error) {
	var res []flux.ControllerStatus
	err := c.Get(ctx, &res, "ListServices", "namespace", namespace)
	return res, err
}

func (c *Client) ListImages(ctx context.Context, s update.ResourceSpec) ([]flux.ImageStatus, error) {
	var res []flux.ImageStatus
	err := c.Get(ctx, &res, "ListImages", "service", string(s))
	return res, err
}

func (c *Client) JobStatus(ctx context.Context, jobID job.ID) (job.Status, error) {
	var res job.Status
	err := c.Get(ctx, &res, "JobStatus", "id", string(jobID))
	return res, err
}

func (c *Client) SyncStatus(ctx context.Context, ref string) ([]string, error) {
	var res []string
	err := c.Get(ctx, &res, "SyncStatus", "ref", ref)
	return res, err
}

func (c *Client) UpdateManifests(ctx context.Context, spec update.Spec) (job.ID, error) {
	var res job.ID
	err := c.methodWithResp(ctx, "POST", &res, "UpdateManifests", spec)
	return res, err
}

func (c *Client) LogEvent(ctx context.Context, event event.Event) error {
	return c.PostWithBody(ctx, "LogEvent", event)
}

func (c *Client) Export(ctx context.Context) ([]byte, error) {
	var res []byte
	err := c.Get(ctx, &res, "Export")
	return res, err
}

func (c *Client) GitRepoConfig(ctx context.Context, regenerate bool) (flux.GitConfig, error) {
	var res flux.GitConfig
	err := c.methodWithResp(ctx, "POST", &res, "GitRepoConfig", regenerate)
	return res, err
}

// --- Request helpers

// post is a simple query-param only post request
func (c *Client) Post(ctx context.Context, route string, queryParams ...string) error {
	return c.PostWithBody(ctx, route, nil, queryParams...)
}

// PostWithBody is a more complex post request, which includes a json-ified body.
// If body is not nil, it is encoded to json before sending
func (c *Client) PostWithBody(ctx context.Context, route string, body interface{}, queryParams ...string) error {
	return c.methodWithResp(ctx, "POST", nil, route, body, queryParams...)
}

func (c *Client) PatchWithBody(ctx context.Context, route string, body interface{}, queryParams ...string) error {
	return c.methodWithResp(ctx, "PATCH", nil, route, body, queryParams...)
}

// methodWithResp is the full enchilada, it handles body and query-param
// encoding, as well as decoding the response into the provided destination.
// Note, the response will only be decoded into the dest if the len is > 0.
func (c *Client) methodWithResp(ctx context.Context, method string, dest interface{}, route string, body interface{}, queryParams ...string) error {
	u, err := transport.MakeURL(c.endpoint, c.router, route, queryParams...)
	if err != nil {
		return errors.Wrap(err, "constructing URL")
	}

	var bodyBytes []byte
	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return errors.Wrap(err, "encoding request body")
		}
	}

	req, err := http.NewRequest(method, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return errors.Wrapf(err, "constructing request %s", u)
	}
	req = req.WithContext(ctx)

	c.token.Set(req)
	req.Header.Set("Accept", "application/json")

	resp, err := c.executeRequest(req)
	if err != nil {
		return errors.Wrap(err, "executing HTTP request")
	}
	defer resp.Body.Close()

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "decoding response from server")
	}
	if len(respBytes) <= 0 {
		return nil
	}
	if err := json.Unmarshal(respBytes, &dest); err != nil {
		return errors.Wrap(err, "decoding response from server")
	}
	return nil
}

// get executes a get request against the flux server. it unmarshals the response into dest.
func (c *Client) Get(ctx context.Context, dest interface{}, route string, queryParams ...string) error {
	u, err := transport.MakeURL(c.endpoint, c.router, route, queryParams...)
	if err != nil {
		return errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return errors.Wrapf(err, "constructing request %s", u)
	}
	req = req.WithContext(ctx)

	c.token.Set(req)
	req.Header.Set("Accept", "application/json")

	resp, err := c.executeRequest(req)
	if err != nil {
		return errors.Wrap(err, "executing HTTP request")
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return errors.Wrap(err, "decoding response from server")
	}
	return nil
}

func (c *Client) executeRequest(req *http.Request) (*http.Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "executing HTTP request")
	}
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		return resp, nil
	case http.StatusUnauthorized:
		return resp, transport.ErrorUnauthorized
	default:
		// Use the content type to discriminate between `fluxerr.Error`,
		// and the previous "any old error"
		if strings.HasPrefix(resp.Header.Get(http.CanonicalHeaderKey("Content-Type")), "application/json") {
			var niceError fluxerr.Error
			if err := json.NewDecoder(resp.Body).Decode(&niceError); err != nil {
				return resp, errors.Wrap(err, "decoding error in response body")
			}
			return resp, &niceError
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return resp, errors.Wrap(err, "reading assumed plaintext response body")
		}
		return resp, errors.New(resp.Status + " " + string(body))
	}
}
