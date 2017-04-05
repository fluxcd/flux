package client

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/api"
	transport "github.com/weaveworks/flux/http"
	"github.com/weaveworks/flux/jobs"
)

type client struct {
	client   *http.Client
	token    flux.Token
	router   *mux.Router
	endpoint string
}

func New(c *http.Client, router *mux.Router, endpoint string, t flux.Token) api.ClientService {
	return &client{
		client:   c,
		token:    t,
		router:   router,
		endpoint: endpoint,
	}
}

func (c *client) ListServices(_ flux.InstanceID, namespace string) ([]flux.ServiceStatus, error) {
	var res []flux.ServiceStatus
	err := c.get(&res, "ListServices", "namespace", namespace)
	return res, err
}

func (c *client) ListImages(_ flux.InstanceID, s flux.ServiceSpec) ([]flux.ImageStatus, error) {
	var res []flux.ImageStatus
	err := c.get(&res, "ListImages", "service", string(s))
	return res, err
}

func (c *client) PostRelease(_ flux.InstanceID, s jobs.ReleaseJobParams) (jobs.JobID, error) {
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

	var resp transport.PostReleaseResponse
	err := c.postWithResp(&resp, "PostRelease", nil, args...)
	return resp.ReleaseID, err
}

func (c *client) GetRelease(_ flux.InstanceID, id jobs.JobID) (jobs.Job, error) {
	var res jobs.Job
	err := c.get(&res, "GetRelease", "id", string(id))
	return res, err
}

func (c *client) Automate(_ flux.InstanceID, id flux.ServiceID) error {
	return c.post("Automate", "service", string(id))
}

func (c *client) Deautomate(_ flux.InstanceID, id flux.ServiceID) error {
	return c.post("Deautomate", "service", string(id))
}

func (c *client) Lock(_ flux.InstanceID, id flux.ServiceID) error {
	return c.post("Lock", "service", string(id))
}

func (c *client) Unlock(_ flux.InstanceID, id flux.ServiceID) error {
	return c.post("Unlock", "service", string(id))
}

func (c *client) History(_ flux.InstanceID, s flux.ServiceSpec) ([]flux.HistoryEntry, error) {
	var res []flux.HistoryEntry
	err := c.get(&res, "History", "service", string(s))
	return res, err
}

func (c *client) GetConfig(_ flux.InstanceID) (flux.InstanceConfig, error) {
	var res flux.InstanceConfig
	err := c.get(&res, "GetConfig")
	return res, err
}

func (c *client) SetConfig(_ flux.InstanceID, config flux.UnsafeInstanceConfig) error {
	return c.postWithBody("SetConfig", config)
}

func (c *client) PatchConfig(_ flux.InstanceID, config flux.ConfigPatch) error {
	return errors.New("not implemented")
}

func (c *client) GenerateDeployKey(_ flux.InstanceID) error {
	return c.post("GenerateDeployKeys")
}

func (c *client) Status(_ flux.InstanceID) (flux.Status, error) {
	var res flux.Status
	err := c.get(&res, "Status")
	return res, err
}

func (c *client) Export(_ flux.InstanceID) ([]byte, error) {
	var res []byte
	err := c.get(&res, "Export")
	return res, err
}

// post is a simple query-param only post request
func (c *client) post(route string, queryParams ...string) error {
	return c.postWithBody(route, nil, queryParams...)
}

// postWithBody is a more complex post request, which includes a json-ified body.
// If body is not nil, it is encoded to json before sending
func (c *client) postWithBody(route string, body interface{}, queryParams ...string) error {
	return c.postWithResp(nil, route, body, queryParams...)
}

// postWithResp is the full enchilada, it handles body and query-param
// encoding, as well as decoding the response into the provided destination.
// Note, the response will only be decoded into the dest if the len is > 0.
func (c *client) postWithResp(dest interface{}, route string, body interface{}, queryParams ...string) error {
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

	req, err := http.NewRequest("POST", u.String(), bytes.NewReader(bodyBytes))
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
func (c *client) get(dest interface{}, route string, queryParams ...string) error {
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

func (c *client) executeRequest(req *http.Request) (*http.Response, error) {
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
