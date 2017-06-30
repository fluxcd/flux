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
)

func DeprecateVersions(r *mux.Router, versions ...string) {
	var deprecated http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, http.StatusGone, ErrorDeprecated)
	}

	// Any versions not represented in the routes below should be
	// deprecated. They are done separately so we can see them as
	// different methods in metrics and logging.
	for _, version := range versions {
		r.NewRoute().Name("Deprecated:" + version).PathPrefix("/" + version + "/").HandlerFunc(deprecated)
	}
}

func NewAPIRouter() *mux.Router {
	r := mux.NewRouter()

	r.NewRoute().Name("ListServices").Methods("GET").Path("/v6/services").Queries("namespace", "{namespace}") // optional namespace!
	r.NewRoute().Name("ListImages").Methods("GET").Path("/v6/images").Queries("service", "{service}")

	r.NewRoute().Name("UpdateImages").Methods("POST").Path("/v6/update-images").Queries("service", "{service}", "image", "{image}", "kind", "{kind}")
	r.NewRoute().Name("UpdatePolicies").Methods("PATCH").Path("/v6/policies")
	r.NewRoute().Name("SyncNotify").Methods("POST").Path("/v6/sync")
	r.NewRoute().Name("JobStatus").Methods("GET").Path("/v6/jobs").Queries("id", "{id}")
	r.NewRoute().Name("SyncStatus").Methods("GET").Path("/v6/sync").Queries("ref", "{ref}")
	r.NewRoute().Name("Export").Methods("HEAD", "GET").Path("/v6/export")
	r.NewRoute().Name("GetPublicSSHKey").Methods("GET").Path("/v6/identity.pub")
	r.NewRoute().Name("RegeneratePublicSSHKey").Methods("POST").Path("/v6/identity.pub")

	return r // TODO 404 though?
}

func UpstreamRoutes(r *mux.Router) {
	r.NewRoute().Name("RegisterDaemon").Methods("GET").Path("/v6/daemon")
	r.NewRoute().Name("LogEvent").Methods("POST").Path("/v6/events")
}

func NewUpstreamRouter() *mux.Router {
	r := mux.NewRouter()
	UpstreamRoutes(r)
	return r
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

func JSONResponse(w http.ResponseWriter, r *http.Request, result interface{}) {
	body, err := json.Marshal(result)
	if err != nil {
		ErrorResponse(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func ErrorResponse(w http.ResponseWriter, r *http.Request, apiError error) {
	var outErr *flux.BaseError
	var code int
	err := errors.Cause(apiError)
	switch err := err.(type) {
	case flux.Missing:
		code = http.StatusNotFound
		outErr = err.BaseError
	case flux.UserConfigProblem:
		code = http.StatusUnprocessableEntity
		outErr = err.BaseError
	case flux.ServerException:
		code = http.StatusInternalServerError
		outErr = err.BaseError
	default:
		code = http.StatusInternalServerError
		outErr = flux.CoverAllError(apiError)
	}

	WriteError(w, r, code, outErr)
}
