package http

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/common/middleware"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/http/error"
	"github.com/weaveworks/flux/http/websocket"
	"github.com/weaveworks/flux/integrations/github"
	"github.com/weaveworks/flux/jobs"
	"github.com/weaveworks/flux/platform"
	"github.com/weaveworks/flux/platform/rpc"
)

func NewRouter() *mux.Router {
	r := mux.NewRouter()
	r.NewRoute().Name("ListServices").Methods("GET").Path("/v3/services").Queries("namespace", "{namespace}") // optional namespace!
	r.NewRoute().Name("ListImages").Methods("GET").Path("/v3/images").Queries("service", "{service}")
	r.NewRoute().Name("PostRelease").Methods("POST").Path("/v4/release").Queries("service", "{service}", "image", "{image}", "kind", "{kind}")
	r.NewRoute().Name("GetRelease").Methods("GET").Path("/v4/release").Queries("id", "{id}")
	r.NewRoute().Name("Automate").Methods("POST").Path("/v3/automate").Queries("service", "{service}")
	r.NewRoute().Name("Deautomate").Methods("POST").Path("/v3/deautomate").Queries("service", "{service}")
	r.NewRoute().Name("Lock").Methods("POST").Path("/v3/lock").Queries("service", "{service}")
	r.NewRoute().Name("Unlock").Methods("POST").Path("/v3/unlock").Queries("service", "{service}")
	r.NewRoute().Name("History").Methods("GET").Path("/v3/history").Queries("service", "{service}")
	r.NewRoute().Name("Status").Methods("GET").Path("/v3/status")
	r.NewRoute().Name("GetConfig").Methods("GET").Path("/v4/config")
	r.NewRoute().Name("SetConfig").Methods("POST").Path("/v4/config")
	r.NewRoute().Name("GenerateDeployKeys").Methods("POST").Path("/v5/config/deploy-keys")
	r.NewRoute().Name("PostIntegrationsGithub").Methods("POST").Path("/v5/integrations/github").Queries("owner", "{owner}", "repository", "{repository}")
	r.NewRoute().Name("RegisterDaemon").Methods("GET").Path("/v4/daemon")
	r.NewRoute().Name("IsConnected").Methods("HEAD", "GET").Path("/v4/ping")
	return r
}

func NewHandler(s api.FluxService, r *mux.Router, logger log.Logger, h *stdprometheus.HistogramVec) http.Handler {
	for method, handlerFunc := range map[string]func(api.FluxService) http.Handler{
		"ListServices":           handleListServices,
		"ListImages":             handleListImages,
		"PostRelease":            handlePostRelease,
		"GetRelease":             handleGetRelease,
		"Automate":               handleAutomate,
		"Deautomate":             handleDeautomate,
		"Lock":                   handleLock,
		"Unlock":                 handleUnlock,
		"History":                handleHistory,
		"Status":                 handleStatus,
		"GetConfig":              handleGetConfig,
		"SetConfig":              handleSetConfig,
		"GenerateDeployKeys":     handleGenerateKeys,
		"PostIntegrationsGithub": handlePostIntegrationsGithub,
		"RegisterDaemon":         handleRegister,
		"IsConnected":            handleIsConnected,
	} {
		var handler http.Handler
		handler = handlerFunc(s)
		handler = logging(handler, log.NewContext(logger).With("method", method))

		r.Get(method).Handler(handler)
	}

	return middleware.Instrument{
		RouteMatcher: r,
		Duration:     h,
	}.Wrap(r)
}

// The idea here is to place the handleFoo and invokeFoo functions next to each
// other, so changes in one can easily be accommodated in the other.

func handleListServices(s api.FluxService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inst := getInstanceID(r)
		namespace := mux.Vars(r)["namespace"]
		res, err := s.ListServices(inst, namespace)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(res); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}
	})
}

func invokeListServices(client *http.Client, t flux.Token, router *mux.Router, endpoint string, namespace string) ([]flux.ServiceStatus, error) {
	u, err := makeURL(endpoint, router, "ListServices", "namespace", namespace)
	if err != nil {
		return nil, errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, errors.Wrapf(err, "constructing request %s", u)
	}
	t.Set(req)

	resp, err := executeRequest(client, req)
	if err != nil {
		return nil, errors.Wrap(err, "executing HTTP request")
	}
	defer resp.Body.Close()

	var res []flux.ServiceStatus
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, errors.Wrap(err, "decoding response from server")
	}
	return res, nil
}

func handleListImages(s api.FluxService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inst := getInstanceID(r)
		service := mux.Vars(r)["service"]
		spec, err := flux.ParseServiceSpec(service)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, errors.Wrapf(err, "parsing service spec %q", service).Error())
			return
		}
		d, err := s.ListImages(inst, spec)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(d); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}
	})
}

func invokeListImages(client *http.Client, t flux.Token, router *mux.Router, endpoint string, s flux.ServiceSpec) ([]flux.ImageStatus, error) {
	u, err := makeURL(endpoint, router, "ListImages", "service", string(s))
	if err != nil {
		return nil, errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, errors.Wrapf(err, "constructing request %s", u)
	}
	t.Set(req)

	resp, err := executeRequest(client, req)
	if err != nil {
		return nil, errors.Wrap(err, "executing HTTP request")
	}
	defer resp.Body.Close()

	var res []flux.ImageStatus
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, errors.Wrap(err, "decoding response from server")
	}
	return res, nil
}

type postReleaseResponse struct {
	Status    string     `json:"status"`
	ReleaseID jobs.JobID `json:"release_id"`
}

func handlePostRelease(s api.FluxService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			inst  = getInstanceID(r)
			vars  = mux.Vars(r)
			image = vars["image"]
			kind  = vars["kind"]
		)
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, errors.Wrapf(err, "parsing form").Error())
			return
		}
		var serviceSpecs []flux.ServiceSpec
		for _, service := range r.Form["service"] {
			serviceSpec, err := flux.ParseServiceSpec(service)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, errors.Wrapf(err, "parsing service spec %q", service).Error())
				return
			}
			serviceSpecs = append(serviceSpecs, serviceSpec)
		}
		imageSpec, err := flux.ParseImageSpec(image)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, errors.Wrapf(err, "parsing image spec %q", image).Error())
			return
		}
		releaseKind, err := flux.ParseReleaseKind(kind)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, errors.Wrapf(err, "parsing release kind %q", kind).Error())
			return
		}

		var excludes []flux.ServiceID
		for _, ex := range r.URL.Query()["exclude"] {
			s, err := flux.ParseServiceID(ex)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, errors.Wrapf(err, "parsing excluded service %q", ex).Error())
				return
			}
			excludes = append(excludes, s)
		}

		id, err := s.PostRelease(inst, jobs.ReleaseJobParams{
			ServiceSpecs: serviceSpecs,
			ImageSpec:    imageSpec,
			Kind:         releaseKind,
			Excludes:     excludes,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(postReleaseResponse{
			Status:    "Queued.",
			ReleaseID: id,
		}); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}
	})
}

func invokePostRelease(client *http.Client, t flux.Token, router *mux.Router, endpoint string, s jobs.ReleaseJobParams) (jobs.JobID, error) {
	args := []string{"image", string(s.ImageSpec), "kind", string(s.Kind)}
	for _, spec := range s.ServiceSpecs {
		args = append(args, "service", string(spec))
	}
	for _, ex := range s.Excludes {
		args = append(args, "exclude", string(ex))
	}

	u, err := makeURL(endpoint, router, "PostRelease", args...)
	if err != nil {
		return "", errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		return "", errors.Wrapf(err, "constructing request %s", u)
	}
	t.Set(req)

	resp, err := executeRequest(client, req)
	if err != nil {
		return "", errors.Wrap(err, "executing HTTP request")
	}
	defer resp.Body.Close()

	var res postReleaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", errors.Wrap(err, "decoding response from server")
	}
	return res.ReleaseID, nil
}

func handleGetRelease(s api.FluxService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inst := getInstanceID(r)
		id := mux.Vars(r)["id"]
		job, err := s.GetRelease(inst, jobs.JobID(id))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(job); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}
	})
}

func invokeGetRelease(client *http.Client, t flux.Token, router *mux.Router, endpoint string, id jobs.JobID) (jobs.Job, error) {
	u, err := makeURL(endpoint, router, "GetRelease", "id", string(id))
	if err != nil {
		return jobs.Job{}, errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return jobs.Job{}, errors.Wrapf(err, "constructing request %s", u)
	}
	t.Set(req)

	resp, err := executeRequest(client, req)
	if err != nil {
		return jobs.Job{}, errors.Wrap(err, "executing HTTP request")
	}
	defer resp.Body.Close()

	var res jobs.Job
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return jobs.Job{}, errors.Wrap(err, "decoding response from server")
	}
	return res, nil
}

func handleAutomate(s api.FluxService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inst := getInstanceID(r)
		service := mux.Vars(r)["service"]
		id, err := flux.ParseServiceID(service)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, errors.Wrapf(err, "parsing service ID %q", id).Error())
			return
		}

		if err = s.Automate(inst, id); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}

func invokeAutomate(client *http.Client, t flux.Token, router *mux.Router, endpoint string, s flux.ServiceID) error {
	u, err := makeURL(endpoint, router, "Automate", "service", string(s))
	if err != nil {
		return errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		return errors.Wrapf(err, "constructing request %s", u)
	}
	t.Set(req)

	resp, err := executeRequest(client, req)
	if err != nil {
		return errors.Wrap(err, "executing HTTP request")
	}
	defer resp.Body.Close()

	return nil
}

func handleDeautomate(s api.FluxService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inst := getInstanceID(r)
		service := mux.Vars(r)["service"]
		id, err := flux.ParseServiceID(service)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, errors.Wrapf(err, "parsing service ID %q", id).Error())
			return
		}

		if err = s.Deautomate(inst, id); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}

func invokeDeautomate(client *http.Client, t flux.Token, router *mux.Router, endpoint string, id flux.ServiceID) error {
	u, err := makeURL(endpoint, router, "Deautomate", "service", string(id))
	if err != nil {
		return errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		return errors.Wrapf(err, "constructing request %s", u)
	}
	t.Set(req)

	resp, err := executeRequest(client, req)
	if err != nil {

		return errors.Wrap(err, "executing HTTP request")
	}
	defer resp.Body.Close()

	return nil
}

func handleLock(s api.FluxService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inst := getInstanceID(r)
		service := mux.Vars(r)["service"]
		id, err := flux.ParseServiceID(service)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, errors.Wrapf(err, "parsing service ID %q", id).Error())
			return
		}

		if err = s.Lock(inst, id); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}

func invokeLock(client *http.Client, t flux.Token, router *mux.Router, endpoint string, id flux.ServiceID) error {
	u, err := makeURL(endpoint, router, "Lock", "service", string(id))
	if err != nil {
		return errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		return errors.Wrapf(err, "constructing request %s", u)
	}
	t.Set(req)

	resp, err := executeRequest(client, req)
	if err != nil {
		return errors.Wrap(err, "executing HTTP request")
	}
	defer resp.Body.Close()

	return nil
}

func handleUnlock(s api.FluxService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inst := getInstanceID(r)
		service := mux.Vars(r)["service"]
		id, err := flux.ParseServiceID(service)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, errors.Wrapf(err, "parsing service ID %q", id).Error())
			return
		}

		if err = s.Unlock(inst, id); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}

func invokeUnlock(client *http.Client, t flux.Token, router *mux.Router, endpoint string, id flux.ServiceID) error {
	u, err := makeURL(endpoint, router, "Unlock", "service", string(id))
	if err != nil {
		return errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		return errors.Wrapf(err, "constructing request %s", u)
	}
	t.Set(req)

	resp, err := executeRequest(client, req)
	if err != nil {
		return errors.Wrap(err, "executing HTTP request")
	}
	defer resp.Body.Close()

	return nil
}

func handleHistory(s api.FluxService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inst := getInstanceID(r)
		service := mux.Vars(r)["service"]
		spec, err := flux.ParseServiceSpec(service)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, errors.Wrapf(err, "parsing service spec %q", spec).Error())
			return
		}

		h, err := s.History(inst, spec)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(h); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}
	})
}

func invokeHistory(client *http.Client, t flux.Token, router *mux.Router, endpoint string, s flux.ServiceSpec) ([]flux.HistoryEntry, error) {
	u, err := makeURL(endpoint, router, "History", "service", string(s))
	if err != nil {
		return nil, errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, errors.Wrapf(err, "constructing request %s", u)
	}
	t.Set(req)

	resp, err := executeRequest(client, req)
	if err != nil {
		return nil, errors.Wrap(err, "executing HTTP request")
	}
	defer resp.Body.Close()

	var res []flux.HistoryEntry
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, errors.Wrap(err, "decoding response from server")
	}

	return res, nil
}

func handleGetConfig(s api.FluxService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inst := getInstanceID(r)
		config, err := s.GetConfig(inst)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		configBytes := bytes.Buffer{}
		if err = json.NewEncoder(&configBytes).Encode(config); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(configBytes.Bytes())
		return
	})
}

func invokeGetConfig(client *http.Client, t flux.Token, router *mux.Router, endpoint string) (flux.InstanceConfig, error) {
	u, err := makeURL(endpoint, router, "GetConfig")
	if err != nil {
		return flux.InstanceConfig{}, errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return flux.InstanceConfig{}, errors.Wrapf(err, "constructing request %s", u)
	}
	t.Set(req)

	resp, err := executeRequest(client, req)
	if err != nil {
		return flux.InstanceConfig{}, errors.Wrap(err, "executing HTTP request")
	}
	defer resp.Body.Close()

	var res flux.InstanceConfig
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return res, errors.Wrap(err, "decoding response body")
	}
	return res, nil
}

func handleSetConfig(s api.FluxService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inst := getInstanceID(r)

		var config flux.UnsafeInstanceConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, err.Error())
			return
		}

		if err := s.SetConfig(inst, config); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.WriteHeader(http.StatusOK)
		return

	})
}

func invokeSetConfig(client *http.Client, t flux.Token, router *mux.Router, endpoint string, updates flux.UnsafeInstanceConfig) error {
	u, err := makeURL(endpoint, router, "SetConfig")
	if err != nil {
		return errors.Wrap(err, "constructing URL")
	}

	var configBytes bytes.Buffer
	if err = json.NewEncoder(&configBytes).Encode(updates); err != nil {
		return errors.Wrap(err, "encoding config updates")
	}

	req, err := http.NewRequest("POST", u.String(), &configBytes)
	if err != nil {
		return errors.Wrapf(err, "constructing request %s", u)
	}
	t.Set(req)

	resp, err := executeRequest(client, req)
	if err != nil {
		return errors.Wrap(err, "executing HTTP request")
	}
	defer resp.Body.Close()

	return nil
}

func handleGenerateKeys(s api.FluxService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inst := getInstanceID(r)
		err := s.GenerateDeployKey(inst)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.WriteHeader(http.StatusOK)
		return
	})
}

func invokeGenerateKeys(client *http.Client, t flux.Token, router *mux.Router, endpoint string) error {
	u, err := makeURL(endpoint, router, "GenerateDeployKeys")
	if err != nil {
		return errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		return errors.Wrapf(err, "constructing request %s", u)
	}
	t.Set(req)

	resp, err := executeRequest(client, req)
	if err != nil {
		return errors.Wrap(err, "executing HTTP request")
	}
	defer resp.Body.Close()
	return nil
}

func handlePostIntegrationsGithub(s api.FluxService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			inst  = getInstanceID(r)
			vars  = mux.Vars(r)
			owner = vars["owner"]
			repo  = vars["repository"]
			tok   = r.Header.Get("GithubToken")
		)

		if repo == "" || owner == "" || tok == "" {
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprintf(w, "repo, owner or token is empty")
			return
		}

		// Generate deploy key
		err := s.GenerateDeployKey(inst)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		// Obtain the generated key
		cfg, err := s.GetConfig(inst)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}
		publicKey := cfg.Git.HideKey().Key

		// Use the Github API to insert the key
		// Have to create a new instance here because there is no
		// clean way of injecting without significantly altering
		// the initialisation (at the top)
		gh := github.NewGithubClient(tok)
		err = gh.InsertDeployKey(owner, repo, publicKey)
		if err != nil {
			httpErr, isHttpErr := err.(*httperror.APIError)
			if isHttpErr {
				w.WriteHeader(httpErr.StatusCode)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
			fmt.Fprintf(w, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	})
}

func invokeStatus(client *http.Client, t flux.Token, router *mux.Router, endpoint string) (flux.Status, error) {
	u, err := makeURL(endpoint, router, "Status")
	if err != nil {
		return flux.Status{}, errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return flux.Status{}, errors.Wrapf(err, "constructing request %s", u)
	}
	t.Set(req)

	resp, err := executeRequest(client, req)
	if err != nil {
		return flux.Status{}, errors.Wrap(err, "executing HTTP request")
	}
	defer resp.Body.Close()

	var res flux.Status
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return res, errors.Wrap(err, "decoding response body")
	}
	return res, nil
}

func handleStatus(s api.FluxService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inst := getInstanceID(r)
		status, err := s.Status(inst)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		statusBytes := bytes.Buffer{}
		if err = json.NewEncoder(&statusBytes).Encode(status); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(statusBytes.Bytes())
		return
	})
}

func handleRegister(s api.FluxService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inst := getInstanceID(r)

		// Upgrade to a websocket
		ws, err := websocket.Upgrade(w, r, nil)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, err.Error())
			return
		}

		// Set up RPC. The service is a websocket _server_ but an RPC
		// _client_.
		rpcClient := rpc.NewClient(ws)

		// Make platform available to clients
		// This should block until the daemon disconnects
		// TODO: Handle the error here
		s.RegisterDaemon(inst, rpcClient)

		// Clean up
		// TODO: Handle the error here
		rpcClient.Close()
	})
}

// invokeRegister, which might be expected here, is supplanted by
// `Daemon.connect()`.

func handleIsConnected(s api.FluxService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inst := getInstanceID(r)

		switch s.IsDaemonConnected(inst) {
		case platform.ErrPlatformNotAvailable:
			w.WriteHeader(http.StatusNotFound)
		case nil:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	})
}

// invokeIsConnected is not implemented, since it is not (at present)
// used in a command-line client command.

// --- end handle/invoke

func mustGetPathTemplate(route *mux.Route) string {
	t, err := route.GetPathTemplate()
	if err != nil {
		panic(err)
	}
	return t
}

func makeURL(endpoint string, router *mux.Router, routeName string, urlParams ...string) (*url.URL, error) {
	if len(urlParams)%2 != 0 {
		panic("urlParams must be even!")
	}

	endpointURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing endpoint %s", endpoint)
	}

	routeURL, err := router.Get(routeName).URL()
	if err != nil {
		return nil, errors.Wrapf(err, "retrieving route path %s", routeName)
	}

	v := url.Values{}
	for i := 0; i < len(urlParams); i += 2 {
		v.Add(urlParams[i], urlParams[i+1])
	}

	endpointURL.Path = path.Join(endpointURL.Path, routeURL.Path)
	endpointURL.RawQuery = v.Encode()
	return endpointURL, nil
}

func getInstanceID(req *http.Request) flux.InstanceID {
	s := req.Header.Get(flux.InstanceIDHeaderKey)
	if s == "" {
		return flux.DefaultInstanceID
	}
	return flux.InstanceID(s)
}

func executeRequest(client *http.Client, req *http.Request) (*http.Response, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "executing HTTP request")
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return resp, nil
	default:
		buf, _ := ioutil.ReadAll(resp.Body)
		body := strings.TrimSpace(string(buf))
		return nil, &httperror.APIError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       body,
		}
	}
}

func logging(next http.Handler, logger log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		begin := time.Now()
		cw := &codeWriter{w, http.StatusOK}
		tw := &teeWriter{cw, bytes.Buffer{}}

		next.ServeHTTP(tw, r)

		requestLogger := log.NewContext(logger).With(
			"url", mustUnescape(r.URL.String()),
			"took", time.Since(begin).String(),
			"status_code", cw.code,
		)
		if cw.code != http.StatusOK {
			requestLogger = requestLogger.With("error", strings.TrimSpace(tw.buf.String()))
		}
		requestLogger.Log()
	})
}

// codeWriter intercepts the HTTP status code. WriteHeader may not be called in
// case of success, so either prepopulate code with http.StatusOK, or check for
// zero on the read side.
type codeWriter struct {
	http.ResponseWriter
	code int
}

func (w *codeWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *codeWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response does not implement http.Hijacker")
	}
	return hj.Hijack()
}

// teeWriter intercepts and stores the HTTP response.
type teeWriter struct {
	http.ResponseWriter
	buf bytes.Buffer
}

func (w *teeWriter) Write(p []byte) (int, error) {
	w.buf.Write(p) // best-effort
	return w.ResponseWriter.Write(p)
}

func (w *teeWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response does not implement http.Hijacker")
	}
	return hj.Hijack()
}

func mustUnescape(s string) string {
	if unescaped, err := url.QueryUnescape(s); err == nil {
		return unescaped
	}
	return s
}
