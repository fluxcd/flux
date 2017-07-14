package http

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/weaveworks/common/middleware"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/history"
	transport "github.com/weaveworks/flux/http"
	"github.com/weaveworks/flux/http/httperror"
	"github.com/weaveworks/flux/http/websocket"
	"github.com/weaveworks/flux/integrations/github"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/remote/rpc"
	"github.com/weaveworks/flux/service"
	"github.com/weaveworks/flux/update"
)

func NewServiceRouter() *mux.Router {
	r := transport.NewAPIRouter()

	transport.DeprecateVersions(r, "v1", "v2")
	transport.UpstreamRoutes(r)

	// Backwards compatibility: we only expect the web UI to use these
	// routes, until it is updated to use v6.
	r.NewRoute().Name("ListServicesV3").Methods("GET").Path("/v3/services").Queries("namespace", "{namespace}") // optional namespace!
	r.NewRoute().Name("ListImagesV3").Methods("GET").Path("/v3/images").Queries("service", "{service}")
	r.NewRoute().Name("UpdatePoliciesV4").Methods("PATCH").Path("/v4/policies")
	r.NewRoute().Name("HistoryV3").Methods("GET").Path("/v3/history").Queries("service", "{service}")
	r.NewRoute().Name("StatusV3").Methods("GET").Path("/v3/status")
	r.NewRoute().Name("GetConfigV4").Methods("GET").Path("/v4/config")
	r.NewRoute().Name("SetConfigV4").Methods("POST").Path("/v4/config")
	r.NewRoute().Name("PatchConfigV4").Methods("PATCH").Path("/v4/config")
	r.NewRoute().Name("ExportV5").Methods("HEAD", "GET").Path("/v5/export")
	r.NewRoute().Name("PostIntegrationsGithubV5").Methods("POST").Path("/v5/integrations/github").Queries("owner", "{owner}", "repository", "{repository}")
	// NB no old IsConnected route, as we expect old requests are
	// forwarded to an instance of the old service, and we want to be
	// able to sniff the daemon version depending on which ping
	// responds.

	// V6 service routes
	r.NewRoute().Name("History").Methods("GET").Path("/v6/history").Queries("service", "{service}")
	r.NewRoute().Name("Status").Methods("GET").Path("/v6/status")
	r.NewRoute().Name("GetConfig").Methods("GET").Path("/v6/config")
	r.NewRoute().Name("SetConfig").Methods("POST").Path("/v6/config")
	r.NewRoute().Name("PatchConfig").Methods("PATCH").Path("/v6/config")
	r.NewRoute().Name("PostIntegrationsGithub").Methods("POST").Path("/v6/integrations/github").Queries("owner", "{owner}", "repository", "{repository}")
	r.NewRoute().Name("IsConnected").Methods("HEAD", "GET").Path("/v6/ping")

	// We assume every request that doesn't match a route is a client
	// calling an old or hitherto unsupported API.
	r.NewRoute().Name("NotFound").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		transport.WriteError(w, r, http.StatusNotFound, transport.MakeAPINotFound(r.URL.Path))
	})

	return r
}

func NewHandler(s api.FluxService, r *mux.Router, logger log.Logger) http.Handler {
	handle := HTTPService{s}
	for method, handlerMethod := range map[string]http.HandlerFunc{
		"ListServices":             handle.ListServices,
		"ListServicesV3":           handle.ListServices,
		"ListImages":               handle.ListImages,
		"ListImagesV3":             handle.ListImages,
		"UpdateImages":             handle.UpdateImages,
		"UpdatePolicies":           handle.UpdatePolicies,
		"UpdatePoliciesV4":         handle.UpdatePolicies,
		"LogEvent":                 handle.LogEvent,
		"History":                  handle.History,
		"HistoryV3":                handle.History,
		"Status":                   handle.Status,
		"StatusV3":                 handle.Status,
		"GetConfigV4":              handle.GetConfig,
		"GetConfig":                handle.GetConfig,
		"SetConfig":                handle.SetConfig,
		"SetConfigV4":              handle.SetConfig,
		"PatchConfig":              handle.PatchConfig,
		"PatchConfigV4":            handle.PatchConfig,
		"PostIntegrationsGithub":   handle.PostIntegrationsGithub,
		"PostIntegrationsGithubV5": handle.PostIntegrationsGithub,
		"Export":                   handle.Export,
		"ExportV5":                 handle.Export,
		"RegisterDaemon":           handle.RegisterV6,
		"IsConnected":              handle.IsConnected,
		"SyncNotify":               handle.SyncNotify,
		"JobStatus":                handle.JobStatus,
		"SyncStatus":               handle.SyncStatus,
		"GetPublicSSHKey":          handle.GetPublicSSHKey,
		"RegeneratePublicSSHKey":   handle.RegeneratePublicSSHKey,
	} {
		handler := logging(handlerMethod, log.NewContext(logger).With("method", method))
		r.Get(method).Handler(handler)
	}

	return middleware.Instrument{
		RouteMatcher: r,
		Duration:     requestDuration,
	}.Wrap(r)
}

type HTTPService struct {
	service api.FluxService
}

func (s HTTPService) ListServices(w http.ResponseWriter, r *http.Request) {
	inst := getInstanceID(r)
	namespace := mux.Vars(r)["namespace"]
	res, err := s.service.ListServices(inst, namespace)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	transport.JSONResponse(w, r, res)
}

func (s HTTPService) ListImages(w http.ResponseWriter, r *http.Request) {
	inst := getInstanceID(r)
	service := mux.Vars(r)["service"]
	spec, err := update.ParseServiceSpec(service)
	if err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, errors.Wrapf(err, "parsing service spec %q", service))
		return
	}

	d, err := s.service.ListImages(inst, spec)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	transport.JSONResponse(w, r, d)
}

func (s HTTPService) UpdateImages(w http.ResponseWriter, r *http.Request) {
	var (
		inst  = getInstanceID(r)
		vars  = mux.Vars(r)
		image = vars["image"]
		kind  = vars["kind"]
	)
	if err := r.ParseForm(); err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, errors.Wrapf(err, "parsing form"))
		return
	}
	var serviceSpecs []update.ServiceSpec
	for _, service := range r.Form["service"] {
		serviceSpec, err := update.ParseServiceSpec(service)
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

	var excludes []flux.ServiceID
	for _, ex := range r.URL.Query()["exclude"] {
		s, err := flux.ParseServiceID(ex)
		if err != nil {
			transport.WriteError(w, r, http.StatusBadRequest, errors.Wrapf(err, "parsing excluded service %q", ex))
			return
		}
		excludes = append(excludes, s)
	}

	jobID, err := s.service.UpdateImages(inst, update.ReleaseSpec{
		ServiceSpecs: serviceSpecs,
		ImageSpec:    imageSpec,
		Kind:         releaseKind,
		Excludes:     excludes,
	}, update.Cause{
		User:    r.FormValue("user"),
		Message: r.FormValue("message"),
	})
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	transport.JSONResponse(w, r, jobID)
}

func (s HTTPService) SyncNotify(w http.ResponseWriter, r *http.Request) {
	instID := getInstanceID(r)
	err := s.service.SyncNotify(instID)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s HTTPService) JobStatus(w http.ResponseWriter, r *http.Request) {
	inst := getInstanceID(r)
	id := job.ID(mux.Vars(r)["id"])
	res, err := s.service.JobStatus(inst, id)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	transport.JSONResponse(w, r, res)
}

func (s HTTPService) SyncStatus(w http.ResponseWriter, r *http.Request) {
	inst := getInstanceID(r)
	rev := mux.Vars(r)["ref"]
	res, err := s.service.SyncStatus(inst, rev)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	transport.JSONResponse(w, r, res)
}

func (s HTTPService) UpdatePolicies(w http.ResponseWriter, r *http.Request) {
	inst := getInstanceID(r)

	var updates policy.Updates
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, err)
		return
	}

	jobID, err := s.service.UpdatePolicies(inst, updates, update.Cause{
		User:    r.FormValue("user"),
		Message: r.FormValue("message"),
	})
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	transport.JSONResponse(w, r, jobID)
}

func (s HTTPService) LogEvent(w http.ResponseWriter, r *http.Request) {
	inst := getInstanceID(r)

	var event history.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, err)
		return
	}

	err := s.service.LogEvent(inst, event)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s HTTPService) History(w http.ResponseWriter, r *http.Request) {
	inst := getInstanceID(r)
	service := mux.Vars(r)["service"]
	spec, err := update.ParseServiceSpec(service)
	if err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, errors.Wrapf(err, "parsing service spec %q", spec))
		return
	}

	before := time.Now().UTC()
	if r.FormValue("before") != "" {
		before, err = time.Parse(time.RFC3339Nano, r.FormValue("before"))
		if err != nil {
			transport.ErrorResponse(w, r, err)
			return
		}
	}
	after := time.Unix(0, 0)
	if r.FormValue("after") != "" {
		after, err = time.Parse(time.RFC3339Nano, r.FormValue("after"))
		if err != nil {
			transport.ErrorResponse(w, r, err)
			return
		}
	}
	limit := int64(-1)
	if r.FormValue("limit") != "" {
		if _, err := fmt.Sscan(r.FormValue("limit"), &limit); err != nil {
			transport.ErrorResponse(w, r, err)
			return
		}
	}

	h, err := s.service.History(inst, spec, before, limit, after)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	if r.FormValue("simple") == "true" {
		// Remove all the individual event data, just return the timestamps and messages
		for i := range h {
			h[i].Event = nil
		}
	}

	transport.JSONResponse(w, r, h)
}

func (s HTTPService) GetConfig(w http.ResponseWriter, r *http.Request) {
	inst := getInstanceID(r)
	fingerprint := r.FormValue("fingerprint")
	config, err := s.service.GetConfig(inst, fingerprint)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	transport.JSONResponse(w, r, config)
}

func (s HTTPService) SetConfig(w http.ResponseWriter, r *http.Request) {
	inst := getInstanceID(r)

	var config service.InstanceConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, err)
		return
	}

	if err := s.service.SetConfig(inst, config); err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s HTTPService) PatchConfig(w http.ResponseWriter, r *http.Request) {
	inst := getInstanceID(r)

	var patch service.ConfigPatch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, err)
		return
	}

	if err := s.service.PatchConfig(inst, patch); err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s HTTPService) PostIntegrationsGithub(w http.ResponseWriter, r *http.Request) {
	var (
		inst  = getInstanceID(r)
		vars  = mux.Vars(r)
		owner = vars["owner"]
		repo  = vars["repository"]
		tok   = r.Header.Get("GithubToken")
	)

	if repo == "" || owner == "" || tok == "" {
		transport.WriteError(w, r, http.StatusUnprocessableEntity, errors.New("repo, owner or token is empty"))
		return
	}

	// Obtain public key from daemon
	publicKey, err := s.service.PublicSSHKey(inst, false)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	// Use the Github API to insert the key
	// Have to create a new instance here because there is no
	// clean way of injecting without significantly altering
	// the initialisation (at the top)
	gh := github.NewGithubClient(tok)
	err = gh.InsertDeployKey(owner, repo, publicKey.Key)
	if err != nil {
		httpErr, isHttpErr := err.(*httperror.APIError)
		code := http.StatusInternalServerError
		if isHttpErr {
			code = httpErr.StatusCode
		}
		transport.WriteError(w, r, code, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s HTTPService) Status(w http.ResponseWriter, r *http.Request) {
	inst := getInstanceID(r)
	status, err := s.service.Status(inst)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	transport.JSONResponse(w, r, status)
}

func (s HTTPService) RegisterV6(w http.ResponseWriter, r *http.Request) {
	s.doRegister(w, r, func(conn io.ReadWriteCloser) platformCloser {
		return rpc.NewClientV6(conn)
	})
}

type platformCloser interface {
	remote.Platform
	io.Closer
}

type platformCloserFn func(io.ReadWriteCloser) platformCloser

func (s HTTPService) doRegister(w http.ResponseWriter, r *http.Request, newRPCFn platformCloserFn) {
	inst := getInstanceID(r)

	// This is not client-facing, so we don't do content
	// negotiation here.

	// Upgrade to a websocket
	ws, err := websocket.Upgrade(w, r, nil)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, err.Error())
		return
	}

	// Set up RPC. The service is a websocket _server_ but an RPC
	// _client_.
	rpcClient := newRPCFn(ws)

	// Make platform available to clients
	// This should block until the daemon disconnects
	// TODO: Handle the error here
	s.service.RegisterDaemon(inst, rpcClient)

	// Clean up
	// TODO: Handle the error here
	rpcClient.Close() // also closes the underlying socket
}

func (s HTTPService) IsConnected(w http.ResponseWriter, r *http.Request) {
	inst := getInstanceID(r)

	err := s.service.IsDaemonConnected(inst)
	if err == nil {
		transport.JSONResponse(w, r, service.FluxdStatus{
			Connected: true,
		})
		return
	}
	switch err.(type) {
	case flux.UserConfigProblem:
		// NB this has a specific contract for "cannot contact" -> // "404 not found"
		transport.WriteError(w, r, http.StatusNotFound, err)
	case flux.Missing: // From standalone, not connected.
		transport.JSONResponse(w, r, service.FluxdStatus{
			Connected: false,
		})
	case remote.FatalError: // An error from nats, but probably due to not connected.
		transport.JSONResponse(w, r, service.FluxdStatus{
			Connected: false,
		})
	default:
		transport.ErrorResponse(w, r, err)
	}
}

func (s HTTPService) Export(w http.ResponseWriter, r *http.Request) {
	inst := getInstanceID(r)
	status, err := s.service.Export(inst)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	transport.JSONResponse(w, r, status)
}

func (s HTTPService) GetPublicSSHKey(w http.ResponseWriter, r *http.Request) {
	inst := getInstanceID(r)
	publicSSHKey, err := s.service.PublicSSHKey(inst, false)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	transport.JSONResponse(w, r, publicSSHKey)
}

func (s HTTPService) RegeneratePublicSSHKey(w http.ResponseWriter, r *http.Request) {
	inst := getInstanceID(r)
	_, err := s.service.PublicSSHKey(inst, true)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	return
}

// --- end handlers

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

func getInstanceID(req *http.Request) service.InstanceID {
	s := req.Header.Get(service.InstanceIDHeaderKey)
	if s == "" {
		return service.NoInstanceID
	}
	return service.InstanceID(s)
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
