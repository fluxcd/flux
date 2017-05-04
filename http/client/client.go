package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/history"
	transport "github.com/weaveworks/flux/http"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/policy"
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

func (c *Client) ListServices(_ flux.InstanceID, namespace string) ([]flux.ServiceStatus, error) {
	var res []flux.ServiceStatus
	err := c.get(&res, "ListServices", "namespace", namespace)
	return res, err
}

func (c *Client) ListImages(_ flux.InstanceID, s update.ServiceSpec) ([]flux.ImageStatus, error) {
	var res []flux.ImageStatus
	err := c.get(&res, "ListImages", "service", string(s))
	return res, err
}

func (c *Client) UpdateImages(_ flux.InstanceID, s update.ReleaseSpec) (job.ID, error) {
	args := []string{
		"image", string(s.ImageSpec),
		"kind", string(s.Kind),
		"user", s.Cause.User,
	}
	for _, spec := range s.ServiceSpecs {
		args = append(args, "service", string(spec))
	}
	for _, ex := range s.Excludes {
		args = append(args, "exclude", string(ex))
	}
	if s.Cause.Message != "" {
		args = append(args, "message", s.Cause.Message)
	}

	var res job.ID
	err := c.methodWithResp("POST", &res, "UpdateImages", nil, args...)
	return res, err
}

func (c *Client) SyncNotify(_ flux.InstanceID) error {
	if err := c.post("SyncNotify"); err != nil {
		return err
	}
	return nil
}

func (c *Client) JobStatus(_ flux.InstanceID, jobID job.ID) (job.Status, error) {
	var res job.Status
	err := c.get(&res, "JobStatus", "id", string(jobID))
	return res, err
}

func (c *Client) SyncStatus(_ flux.InstanceID, ref string) ([]string, error) {
	var res []string
	err := c.get(&res, "SyncStatus", "ref", ref)
	return res, err
}

func (c *Client) UpdatePolicies(_ flux.InstanceID, updates policy.Updates) (job.ID, error) {
	var res job.ID
	return res, c.methodWithResp("PATCH", &res, "UpdatePolicies", updates)
}

func (c *Client) LogEvent(_ flux.InstanceID, event history.Event) error {
	return c.postWithBody("LogEvent", event)
}

func (c *Client) History(_ flux.InstanceID, s update.ServiceSpec, before time.Time, limit int64) ([]history.Entry, error) {
	params := []string{"service", string(s)}
	if !before.IsZero() {
		params = append(params, "before", before.Format(time.RFC3339Nano))
	}
	if limit >= 0 {
		params = append(params, "limit", fmt.Sprint(limit))
	}
	var res []history.Entry
	err := c.get(&res, "History", params...)
	return res, err
}

func (c *Client) GetConfig(_ flux.InstanceID, fingerprint string) (flux.InstanceConfig, error) {
	var params []string
	if fingerprint != "" {
		params = append(params, "fingerprint", fingerprint)
	}
	var res flux.InstanceConfig
	err := c.get(&res, "GetConfig", params...)
	return res, err
}

func (c *Client) SetConfig(_ flux.InstanceID, config flux.UnsafeInstanceConfig) error {
	return c.postWithBody("SetConfig", config)
}

func (c *Client) PatchConfig(_ flux.InstanceID, patch flux.ConfigPatch) error {
	return c.patchWithBody("PatchConfig", patch)
}

func (c *Client) GenerateDeployKey(_ flux.InstanceID) error {
	return c.post("GenerateDeployKeys")
}

func (c *Client) Status(_ flux.InstanceID) (flux.Status, error) {
	var res flux.Status
	err := c.get(&res, "Status")
	return res, err
}

func (c *Client) Export(_ flux.InstanceID) ([]byte, error) {
	var res []byte
	err := c.get(&res, "Export")
	return res, err
}

// post is a simple query-param only post request
func (c *Client) post(route string, queryParams ...string) error {
	return c.postWithBody(route, nil, queryParams...)
}

// postWithBody is a more complex post request, which includes a json-ified body.
// If body is not nil, it is encoded to json before sending
func (c *Client) postWithBody(route string, body interface{}, queryParams ...string) error {
	return c.methodWithResp("POST", nil, route, body, queryParams...)
}

func (c *Client) patchWithBody(route string, body interface{}, queryParams ...string) error {
	return c.methodWithResp("PATCH", nil, route, body, queryParams...)
}

// methodWithResp is the full enchilada, it handles body and query-param
// encoding, as well as decoding the response into the provided destination.
// Note, the response will only be decoded into the dest if the len is > 0.
func (c *Client) methodWithResp(method string, dest interface{}, route string, body interface{}, queryParams ...string) error {
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
func (c *Client) get(dest interface{}, route string, queryParams ...string) error {
	u, err := transport.MakeURL(c.endpoint, c.router, route, queryParams...)
	if err != nil {
		return errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return errors.Wrapf(err, "constructing request %s", u)
	}
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
		// Use the content type to discriminate between `flux.BaseError`,
		// and the previous "any old error"
		if strings.HasPrefix(resp.Header.Get(http.CanonicalHeaderKey("Content-Type")), "application/json") {
			var niceError flux.BaseError
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
