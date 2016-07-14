// Package api implements the REST-y API for fluxd.
package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"github.com/weaveworks/fluxy/platform/kubernetes"
)

// Server implements the fluxd API as consumed by the fluxctl client.
type Server struct {
	Platform *kubernetes.Cluster // TODO(pb): replace with platform.Platform when we have that
}

// ListenAndServe mirrors http.Server and should be invoked in func main.
func (s *Server) ListenAndServe(addr string) error {
	r := mux.NewRouter()
	s.installRoutes(r)
	return http.ListenAndServe(addr, r)
}

func (s *Server) installRoutes(r *mux.Router) {
	r.StrictSlash(false)
	r.Methods("GET").Path("/").HandlerFunc(s.status)
	v0 := r.PathPrefix("/v0").Subrouter()
	v0.Methods("GET").Path("/services").HandlerFunc(s.services)
	v0.Methods("POST").Path("/release").HandlerFunc(s.release)
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	s.respond(w, http.StatusOK, map[string]string{"status": "OK"})
}

func (s *Server) services(w http.ResponseWriter, r *http.Request) {
	if s.Platform == nil {
		s.respond(w, http.StatusInternalServerError, errorBody("the platform is not configured"))
		return
	}

	namespace := r.FormValue("namespace")
	if namespace == "" {
		namespace = "default"
	}

	services, err := s.Platform.Services(namespace)
	if err != nil {
		s.respond(w, http.StatusNotFound, errorBody(err))
		return
	}

	s.respond(w, http.StatusOK, services)
}

func (s *Server) release(w http.ResponseWriter, r *http.Request) {
	if s.Platform == nil {
		s.respond(w, http.StatusInternalServerError, errorBody("the platform is not configured"))
		return
	}

	namespace := r.FormValue("namespace")
	if namespace == "" {
		namespace = "default"
	}

	serviceName := r.FormValue("service")
	if serviceName == "" {
		s.respond(w, http.StatusBadRequest, errorBody("service parameter is required"))
		return
	}

	if r.ContentLength <= 1 {
		s.respond(w, http.StatusBadRequest, errorBody("provide the new RC definition in the request body"))
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		s.respond(w, http.StatusInternalServerError, errorBody(err))
		return
	}
	r.Body.Close()

	updatePeriodStr := r.FormValue("updatePeriod")
	if updatePeriodStr == "" {
		updatePeriodStr = "5s"
	}
	updatePeriod, err := time.ParseDuration(updatePeriodStr)
	if err != nil {
		s.respond(w, http.StatusBadRequest, errorBody(err))
		return
	}

	code, resp := http.StatusOK, map[string]interface{}{
		"namespace":    namespace,
		"serviceName":  serviceName,
		"updatePeriod": updatePeriod.String(),
		"success":      true,
	}
	if err = s.Platform.Release(namespace, serviceName, body, updatePeriod); err != nil {
		resp["success"] = false
		resp["err"] = err.Error()
		code = http.StatusInternalServerError
	}
	s.respond(w, code, resp)
}

func (s *Server) respond(w http.ResponseWriter, code int, x interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(x)
}

func errorBody(err interface{}) interface{} {
	switch x := err.(type) {
	case error:
		return map[string]interface{}{"err": x.Error()}
	case fmt.Stringer:
		return map[string]interface{}{"err": x.String()}
	default:
		return map[string]interface{}{"err": x}
	}
}
