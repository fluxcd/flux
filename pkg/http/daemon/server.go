package daemon

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	stdprometheus "github.com/prometheus/client_golang/prometheus"

	"github.com/fluxcd/flux/pkg/api"
	"github.com/fluxcd/flux/pkg/api/v10"
	"github.com/fluxcd/flux/pkg/api/v11"
	"github.com/fluxcd/flux/pkg/api/v9"
	transport "github.com/fluxcd/flux/pkg/http"
	"github.com/fluxcd/flux/pkg/job"
	fluxmetrics "github.com/fluxcd/flux/pkg/metrics"
	"github.com/fluxcd/flux/pkg/resource"
	"github.com/fluxcd/flux/pkg/update"
	"github.com/weaveworks/common/middleware"
)

var (
	requestDuration = stdprometheus.NewHistogramVec(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Name:      "request_duration_seconds",
		Help:      "Time (in seconds) spent serving HTTP requests.",
		Buckets:   stdprometheus.DefBuckets,
	}, []string{fluxmetrics.LabelMethod, fluxmetrics.LabelRoute, "status_code", "ws"})
)

// An API server for the daemon
func NewRouter() *mux.Router {
	r := transport.NewAPIRouter()

	// All old versions are deprecated in the daemon. Use an up to
	// date client!
	transport.DeprecateVersions(r, "v1", "v2", "v3", "v4", "v5")
	// We assume every request that doesn't match a route is a client
	// calling an old or hitherto unsupported API.
	r.NewRoute().Name("NotFound").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		transport.WriteError(w, r, http.StatusNotFound, transport.MakeAPINotFound(r.URL.Path))
	})

	return r
}

func NewHandler(s api.Server, r *mux.Router) http.Handler {
	handle := HTTPServer{s}

	// Erstwhile Upstream(Server) methods, now part of v11
	r.Get(transport.Ping).HandlerFunc(handle.Ping)
	r.Get(transport.Version).HandlerFunc(handle.Version)
	r.Get(transport.Notify).HandlerFunc(handle.Notify)

	// v6-v11 handlers
	r.Get(transport.ListServices).HandlerFunc(handle.ListServicesWithOptions)
	r.Get(transport.ListServicesWithOptions).HandlerFunc(handle.ListServicesWithOptions)
	r.Get(transport.ListImages).HandlerFunc(handle.ListImagesWithOptions)
	r.Get(transport.ListImagesWithOptions).HandlerFunc(handle.ListImagesWithOptions)
	r.Get(transport.UpdateManifests).HandlerFunc(handle.UpdateManifests)
	r.Get(transport.JobStatus).HandlerFunc(handle.JobStatus)
	r.Get(transport.SyncStatus).HandlerFunc(handle.SyncStatus)
	r.Get(transport.Export).HandlerFunc(handle.Export)
	r.Get(transport.GitRepoConfig).HandlerFunc(handle.GitRepoConfig)

	// These handlers persist to support requests from older fluxctls. In general we
	// should avoid adding references to them so that they can eventually be removed.
	r.Get(transport.UpdateImages).HandlerFunc(handle.UpdateImages)
	r.Get(transport.UpdatePolicies).HandlerFunc(handle.UpdatePolicies)
	r.Get(transport.GetPublicSSHKey).HandlerFunc(handle.GetPublicSSHKey)
	r.Get(transport.RegeneratePublicSSHKey).HandlerFunc(handle.RegeneratePublicSSHKey)

	return middleware.Instrument{
		RouteMatcher: r,
		Duration:     requestDuration,
	}.Wrap(r)
}

type HTTPServer struct {
	server api.Server
}

func (s HTTPServer) Ping(w http.ResponseWriter, r *http.Request) {
	if err := s.server.Ping(r.Context()); err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	return
}

func (s HTTPServer) Version(w http.ResponseWriter, r *http.Request) {
	version, err := s.server.Version(r.Context())
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	transport.JSONResponse(w, r, version)
}

func (s HTTPServer) Notify(w http.ResponseWriter, r *http.Request) {
	var change v9.Change
	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&change); err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, err)
		return
	}
	if err := s.server.NotifyChange(r.Context(), change); err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s HTTPServer) JobStatus(w http.ResponseWriter, r *http.Request) {
	id := job.ID(mux.Vars(r)["id"])
	status, err := s.server.JobStatus(r.Context(), id)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	transport.JSONResponse(w, r, status)
}

func (s HTTPServer) SyncStatus(w http.ResponseWriter, r *http.Request) {
	ref := mux.Vars(r)["ref"]
	commits, err := s.server.SyncStatus(r.Context(), ref)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	transport.JSONResponse(w, r, commits)
}

func (s HTTPServer) ListImagesWithOptions(w http.ResponseWriter, r *http.Request) {
	var opts v10.ListImagesOptions
	queryValues := r.URL.Query()

	// service - Select services to list images for.
	service := queryValues.Get("service")
	if service == "" {
		service = string(update.ResourceSpecAll)
	}
	spec, err := update.ParseResourceSpec(service)
	if err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, errors.Wrapf(err, "parsing service spec %q", service))
		return
	}
	opts.Spec = spec

	// containerFields - Override which fields to return in the container struct.
	containerFields := queryValues.Get("containerFields")
	if containerFields != "" {
		opts.OverrideContainerFields = strings.Split(containerFields, ",")
	}

	// namespace - Select namespace to list images for.
	namespace := queryValues.Get("namespace")
	if namespace != "" {
		opts.Namespace = namespace
	}

	d, err := s.server.ListImagesWithOptions(r.Context(), opts)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	transport.JSONResponse(w, r, d)
}

func (s HTTPServer) UpdateManifests(w http.ResponseWriter, r *http.Request) {
	var spec update.Spec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, err)
		return
	}

	jobID, err := s.server.UpdateManifests(r.Context(), spec)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	transport.JSONResponse(w, r, jobID)
}

func (s HTTPServer) ListServicesWithOptions(w http.ResponseWriter, r *http.Request) {
	var opts v11.ListServicesOptions
	opts.Namespace = r.URL.Query().Get("namespace")
	services := r.URL.Query().Get("services")
	if services != "" {
		for _, svc := range strings.Split(services, ",") {
			id, err := resource.ParseID(svc)
			if err != nil {
				transport.WriteError(w, r, http.StatusBadRequest, errors.Wrapf(err, "parsing service spec %q", svc))
				return
			}
			opts.Services = append(opts.Services, id)
		}
	}

	res, err := s.server.ListServicesWithOptions(r.Context(), opts)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	transport.JSONResponse(w, r, res)
}

func (s HTTPServer) Export(w http.ResponseWriter, r *http.Request) {
	status, err := s.server.Export(r.Context())
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	transport.JSONResponse(w, r, status)
}

func (s HTTPServer) GitRepoConfig(w http.ResponseWriter, r *http.Request) {
	var regenerate bool
	if err := json.NewDecoder(r.Body).Decode(&regenerate); err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, err)
	}
	res, err := s.server.GitRepoConfig(r.Context(), regenerate)
	if err != nil {
		transport.ErrorResponse(w, r, err)
	}
	transport.JSONResponse(w, r, res)
}

// --- handlers supporting deprecated requests

func (s HTTPServer) UpdateImages(w http.ResponseWriter, r *http.Request) {
	var (
		vars  = mux.Vars(r)
		image = vars["image"]
		kind  = vars["kind"]
	)
	if err := r.ParseForm(); err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, errors.Wrapf(err, "parsing form"))
		return
	}
	var serviceSpecs []update.ResourceSpec
	for _, service := range r.Form["service"] {
		serviceSpec, err := update.ParseResourceSpec(service)
		if err != nil {
			transport.WriteError(w, r, http.StatusBadRequest, errors.Wrapf(err, "parsing service spec %q", service))
			return
		}
		serviceSpecs = append(serviceSpecs, serviceSpec)
	}
	imageSpec, err := update.ParseImageSpec(image)
	if err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, errors.Wrapf(err, "parsing image spec %q", image))
		return
	}
	releaseKind, err := update.ParseReleaseKind(kind)
	if err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, errors.Wrapf(err, "parsing release kind %q", kind))
		return
	}

	var excludes []resource.ID
	for _, ex := range r.URL.Query()["exclude"] {
		s, err := resource.ParseID(ex)
		if err != nil {
			transport.WriteError(w, r, http.StatusBadRequest, errors.Wrapf(err, "parsing excluded service %q", ex))
			return
		}
		excludes = append(excludes, s)
	}

	spec := update.ReleaseImageSpec{
		ServiceSpecs: serviceSpecs,
		ImageSpec:    imageSpec,
		Kind:         releaseKind,
		Excludes:     excludes,
	}
	cause := update.Cause{
		User:    r.FormValue("user"),
		Message: r.FormValue("message"),
	}
	result, err := s.server.UpdateManifests(r.Context(), update.Spec{Type: update.Images, Cause: cause, Spec: spec})
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	transport.JSONResponse(w, r, result)
}

func (s HTTPServer) UpdatePolicies(w http.ResponseWriter, r *http.Request) {
	var updates resource.PolicyUpdates
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, err)
		return
	}

	cause := update.Cause{
		User:    r.FormValue("user"),
		Message: r.FormValue("message"),
	}

	jobID, err := s.server.UpdateManifests(r.Context(), update.Spec{Type: update.Policy, Cause: cause, Spec: updates})
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	transport.JSONResponse(w, r, jobID)
}

func (s HTTPServer) GetPublicSSHKey(w http.ResponseWriter, r *http.Request) {
	res, err := s.server.GitRepoConfig(r.Context(), false)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	transport.JSONResponse(w, r, res.PublicSSHKey)
}

func (s HTTPServer) RegeneratePublicSSHKey(w http.ResponseWriter, r *http.Request) {
	_, err := s.server.GitRepoConfig(r.Context(), true)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	return
}
