package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	fluxerr "github.com/weaveworks/flux/errors"
	"github.com/weaveworks/flux/history"
	transport "github.com/weaveworks/flux/http"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/ssh"
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

func (c *Client) ListServices(ctx context.Context, namespace string) ([]flux.ServiceStatus, error) {
	var res []flux.ServiceStatus
	err := c.get(&res, "ListServices", "namespace", namespace)
	return res, err
}

func (c *Client) ListImages(ctx context.Context, s update.ServiceSpec) ([]flux.ImageStatus, error) {
	var res []flux.ImageStatus
	err := c.get(&res, "ListImages", "service", string(s))
	return res, err
}

func (c *Client) UpdateImages(ctx context.Context, s update.ReleaseSpec, cause update.Cause) (job.ID, error) {
	args := []string{
		"image", string(s.ImageSpec),
		"kind", string(s.Kind),
		"user", cause.User,
	}
	for _, spec := range s.ServiceSpecs {
		args = append(args, "service", string(spec))
	}
	for _, ex := range s.Excludes {
		args = append(args, "exclude", ex.String())
	}
	if cause.Message != "" {
		args = append(args, "message", cause.Message)
	}

	var res job.ID
	err := c.methodWithResp("POST", &res, "UpdateImages", nil, args...)
	return res, err
}

func (c *Client) SyncNotify(ctx context.Context) error {
	if err := c.post("SyncNotify"); err != nil {
		return err
	}
	return nil
}

func (c *Client) JobStatus(ctx context.Context, jobID job.ID) (job.Status, error) {
	var res job.Status
	err := c.get(&res, "JobStatus", "id", string(jobID))
	return res, err
}

func (c *Client) SyncStatus(ctx context.Context, ref string) ([]string, error) {
	var res []string
	err := c.get(&res, "SyncStatus", "ref", ref)
	return res, err
}

func (c *Client) UpdatePolicies(ctx context.Context, updates policy.Updates, cause update.Cause) (job.ID, error) {
	args := []string{"user", cause.User}
	if cause.Message != "" {
		args = append(args, "message", cause.Message)
	}
	var res job.ID
	return res, c.methodWithResp("PATCH", &res, "UpdatePolicies", updates, args...)
}

func (c *Client) LogEvent(ctx context.Context, event history.Event) error {
	return c.postWithBody("LogEvent", event)
}

func (c *Client) History(ctx context.Context, s update.ServiceSpec, before time.Time, limit int64, after time.Time) ([]history.Entry, error) {
	params := []string{"service", string(s)}
	if !before.IsZero() {
		params = append(params, "before", before.Format(time.RFC3339Nano))
	}
	if !after.IsZero() {
		params = append(params, "after", after.Format(time.RFC3339Nano))
	}
	if limit >= 0 {
		params = append(params, "limit", fmt.Sprint(limit))
	}
	var res []history.Entry
	err := c.get(&res, "History", params...)
	return res, err
}

func (c *Client) Export(ctx context.Context) ([]byte, error) {
	var res []byte
	err := c.get(&res, "Export")
	return res, err
}

func (c *Client) PublicSSHKey(ctx context.Context, regenerate bool) (ssh.PublicKey, error) {
	if regenerate {
		err := c.post("RegeneratePublicSSHKey")
		if err != nil {
			return ssh.PublicKey{}, err
		}
	}

	var res ssh.PublicKey
	err := c.get(&res, "GetPublicSSHKey")
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
