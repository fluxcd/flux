package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/jobs"
)

func NewRouter() *mux.Router {
	r := mux.NewRouter()
	// Any versions not represented in the routes below are
	// deprecated. They are done separately so we can see them as
	// different methods in metrics and logging.
	for _, version := range []string{"v1", "v2"} {
		r.NewRoute().Name("Deprecated:" + version).PathPrefix("/" + version + "/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			WriteError(w, r, http.StatusGone, ErrorDeprecated)
		})
	}

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
	r.NewRoute().Name("PatchConfig").Methods("PATCH").Path("/v4/config")
	r.NewRoute().Name("GenerateDeployKeys").Methods("POST").Path("/v5/config/deploy-keys")
	r.NewRoute().Name("PostIntegrationsGithub").Methods("POST").Path("/v5/integrations/github").Queries("owner", "{owner}", "repository", "{repository}")
	r.NewRoute().Name("RegisterDaemonV4").Methods("GET").Path("/v4/daemon")
	r.NewRoute().Name("RegisterDaemonV5").Methods("GET").Path("/v5/daemon")
	r.NewRoute().Name("IsConnected").Methods("HEAD", "GET").Path("/v4/ping")
	r.NewRoute().Name("Export").Methods("HEAD", "GET").Path("/v5/export")

	// We assume every request that doesn't match a route is a client
	// calling an old or hitherto unsupported API.
	r.NewRoute().Name("NotFound").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, http.StatusNotFound, MakeAPINotFound(r.URL.Path))
	})

	return r
}

type PostReleaseResponse struct {
	Status    string     `json:"status"`
	ReleaseID jobs.JobID `json:"release_id"`
}

func MakeURL(endpoint string, router *mux.Router, routeName string, urlParams ...string) (*url.URL, error) {
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

func WriteError(w http.ResponseWriter, r *http.Request, code int, err error) {
	// An Accept header with "application/json" is sent by clients
	// understanding how to decode JSON errors. Older clients don't
	// send an Accept header, so we just give them the error text.
	if len(r.Header.Get("Accept")) > 0 {
		switch negotiateContentType(r, []string{"application/json", "text/plain"}) {
		case "application/json":
			body, encodeErr := json.Marshal(err)
			if encodeErr != nil {
				w.Header().Set(http.CanonicalHeaderKey("Content-Type"), "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "Error encoding error response: %s\n\nOriginal error: %s", encodeErr.Error(), err.Error())
				return
			}
			w.Header().Set(http.CanonicalHeaderKey("Content-Type"), "application/json; charset=utf-8")
			w.WriteHeader(code)
			w.Write(body)
			return
		case "text/plain":
			w.Header().Set(http.CanonicalHeaderKey("Content-Type"), "text/plain; charset=utf-8")
			w.WriteHeader(code)
			switch err := err.(type) {
			case *flux.BaseError:
				fmt.Fprint(w, err.Help)
			default:
				fmt.Fprint(w, err.Error())
			}
			return
		}
	}
	w.Header().Set(http.CanonicalHeaderKey("Content-Type"), "text/plain; charset=utf-8")
	w.WriteHeader(code)
	fmt.Fprint(w, err.Error())
}
