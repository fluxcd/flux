package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	fluxerr "github.com/fluxcd/flux/pkg/errors"
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

	r.NewRoute().Name(Ping).Methods("GET").Path("/v11/ping")
	r.NewRoute().Name(Version).Methods("GET").Path("/v11/version")
	r.NewRoute().Name(Notify).Methods("POST").Path("/v11/notify")

	r.NewRoute().Name(ListServices).Methods("GET").Path("/v6/services")
	r.NewRoute().Name(ListServicesWithOptions).Methods("GET").Path("/v11/services")
	r.NewRoute().Name(ListImages).Methods("GET").Path("/v6/images")
	r.NewRoute().Name(ListImagesWithOptions).Methods("GET").Path("/v10/images")

	r.NewRoute().Name(UpdateManifests).Methods("POST").Path("/v9/update-manifests")
	r.NewRoute().Name(JobStatus).Methods("GET").Path("/v6/jobs").Queries("id", "{id}")
	r.NewRoute().Name(SyncStatus).Methods("GET").Path("/v6/sync").Queries("ref", "{ref}")
	r.NewRoute().Name(Export).Methods("HEAD", "GET").Path("/v6/export")
	r.NewRoute().Name(GitRepoConfig).Methods("POST").Path("/v9/git-repo-config")

	// These routes persist to support requests from older fluxctls. In general we
	// should avoid adding references to them so that they can eventually be removed.
	r.NewRoute().Name(UpdateImages).Methods("POST").Path("/v6/update-images").Queries("service", "{service}", "image", "{image}", "kind", "{kind}")
	r.NewRoute().Name(UpdatePolicies).Methods("PATCH").Path("/v6/policies")
	r.NewRoute().Name(GetPublicSSHKey).Methods("GET").Path("/v6/identity.pub")
	r.NewRoute().Name(RegeneratePublicSSHKey).Methods("POST").Path("/v6/identity.pub")

	return r // TODO 404 though?
}

// These (routes, and constructor following) should move to
// weaveworks/flux-adapter when `--connect` is removed from fluxd.
func UpstreamRoutes(r *mux.Router) {
	r.NewRoute().Name(RegisterDaemonV6).Methods("GET").Path("/v6/daemon")
	r.NewRoute().Name(RegisterDaemonV7).Methods("GET").Path("/v7/daemon")
	r.NewRoute().Name(RegisterDaemonV8).Methods("GET").Path("/v8/daemon")
	r.NewRoute().Name(RegisterDaemonV9).Methods("GET").Path("/v9/daemon")
	r.NewRoute().Name(RegisterDaemonV10).Methods("GET").Path("/v10/daemon")
	r.NewRoute().Name(RegisterDaemonV11).Methods("GET").Path("/v11/daemon")
	r.NewRoute().Name(LogEvent).Methods("POST").Path("/v6/events")
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
	route := router.Get(routeName)
	if route == nil {
		return nil, errors.New("no route with name " + routeName)
	}
	routeURL, err := route.URLPath()
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
			case *fluxerr.Error:
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
	var outErr *fluxerr.Error
	var code int
	var ok bool

	err := errors.Cause(apiError)
	if outErr, ok = err.(*fluxerr.Error); !ok {
		outErr = fluxerr.CoverAllError(apiError)
	}
	switch outErr.Type {
	case fluxerr.Missing:
		code = http.StatusNotFound
	case fluxerr.User:
		code = http.StatusUnprocessableEntity
	case fluxerr.Server:
		code = http.StatusInternalServerError
	default:
		code = http.StatusInternalServerError
	}
	WriteError(w, r, code, outErr)
}
