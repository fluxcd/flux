package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	"github.com/fluxcd/flux/pkg/api"
	"github.com/fluxcd/flux/pkg/api/v10"
	"github.com/fluxcd/flux/pkg/api/v11"
	"github.com/fluxcd/flux/pkg/api/v6"
	"github.com/fluxcd/flux/pkg/api/v9"
	fluxerr "github.com/fluxcd/flux/pkg/errors"
	"github.com/fluxcd/flux/pkg/event"
	transport "github.com/fluxcd/flux/pkg/http"
	"github.com/fluxcd/flux/pkg/job"
	"github.com/fluxcd/flux/pkg/update"
)

var (
	errNotImplemented = errors.New("not implemented")
)

type Token string

func (t Token) Set(req *http.Request) {
	if string(t) != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Scope-Probe token=%s", t))
	}
}

type Client struct {
	client   *http.Client
	token    Token
	router   *mux.Router
	endpoint string
}

var _ api.Server = &Client{}

func New(c *http.Client, router *mux.Router, endpoint string, t Token) *Client {
	return &Client{
		client:   c,
		token:    t,
		router:   router,
		endpoint: endpoint,
	}
}

func (c *Client) Ping(ctx context.Context) error {
	return c.Get(ctx, nil, transport.Ping)
}

func (c *Client) Version(ctx context.Context) (string, error) {
	var v string
	err := c.Get(ctx, &v, transport.Version)
	return v, err
}

func (c *Client) NotifyChange(ctx context.Context, change v9.Change) error {
	return c.PostWithBody(ctx, transport.Notify, change)
}

func (c *Client) ListServices(ctx context.Context, namespace string) ([]v6.ControllerStatus, error) {
	var res []v6.ControllerStatus
	err := c.Get(ctx, &res, transport.ListServices, "namespace", namespace)
	return res, err
}

func (c *Client) ListServicesWithOptions(ctx context.Context, opts v11.ListServicesOptions) ([]v6.ControllerStatus, error) {
	var res []v6.ControllerStatus
	var services []string
	for _, svc := range opts.Services {
		services = append(services, svc.String())
	}
	err := c.Get(ctx, &res, transport.ListServicesWithOptions, "namespace", opts.Namespace, "services", strings.Join(services, ","))
	return res, err
}

func (c *Client) ListImages(ctx context.Context, s update.ResourceSpec) ([]v6.ImageStatus, error) {
	var res []v6.ImageStatus
	err := c.Get(ctx, &res, transport.ListImages, "service", string(s))
	return res, err
}

func (c *Client) ListImagesWithOptions(ctx context.Context, opts v10.ListImagesOptions) ([]v6.ImageStatus, error) {
	var res []v6.ImageStatus
	err := c.Get(ctx, &res, transport.ListImagesWithOptions, "service", string(opts.Spec), "containerFields", strings.Join(opts.OverrideContainerFields, ","), "namespace", opts.Namespace)
	return res, err
}

func (c *Client) JobStatus(ctx context.Context, jobID job.ID) (job.Status, error) {
	var res job.Status
	err := c.Get(ctx, &res, transport.JobStatus, "id", string(jobID))
	return res, err
}

func (c *Client) SyncStatus(ctx context.Context, ref string) ([]string, error) {
	var res []string
	err := c.Get(ctx, &res, transport.SyncStatus, "ref", ref)
	return res, err
}

func (c *Client) UpdateManifests(ctx context.Context, spec update.Spec) (job.ID, error) {
	var res job.ID
	err := c.methodWithResp(ctx, "POST", &res, transport.UpdateManifests, spec)
	return res, err
}

func (c *Client) LogEvent(ctx context.Context, event event.Event) error {
	return c.PostWithBody(ctx, transport.LogEvent, event)
}

func (c *Client) Export(ctx context.Context) ([]byte, error) {
	var res []byte
	err := c.Get(ctx, &res, transport.Export)
	return res, err
}

func (c *Client) GitRepoConfig(ctx context.Context, regenerate bool) (v6.GitConfig, error) {
	var res v6.GitConfig
	err := c.methodWithResp(ctx, "POST", &res, transport.GitRepoConfig, regenerate)
	return res, err
}

// --- Request helpers

// Post is a simple query-param only post request
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

// Get executes a get request against the Flux server. it unmarshals the response into dest, if not nil.
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

	if dest != nil {
		if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
			return errors.Wrap(err, "decoding response from server")
		}
	}
	return nil
}

func (c *Client) executeRequest(req *http.Request) (*http.Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "executing HTTP request")
	}
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent, http.StatusAccepted:
		return resp, nil
	case http.StatusUnauthorized:
		return resp, transport.ErrorUnauthorized
	default:
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return resp, errors.Wrap(err, "reading response body of error")
		}
		// Use the content type to discriminate between `fluxerr.Error`,
		// and the previous "any old error"
		if strings.HasPrefix(resp.Header.Get(http.CanonicalHeaderKey("Content-Type")), "application/json") {
			var niceError fluxerr.Error
			if err := json.Unmarshal(body, &niceError); err != nil {
				return resp, errors.Wrap(err, "decoding response body of error")
			}
			// just in case it's JSON but not one of our own errors
			if niceError.Err != nil {
				return resp, &niceError
			}
			// fallthrough
		}
		return resp, errors.New(resp.Status + " " + string(body))
	}
}
