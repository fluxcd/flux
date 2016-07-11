package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

type APIServer struct {
}

func (srv *APIServer) ListenAndServe(addr string) error {
	r := mux.NewRouter()
	srv.installRoutes(r)
	return http.ListenAndServe(addr, r)
}

func (srv *APIServer) installRoutes(r *mux.Router) {
	r.HandleFunc("/", srv.status)
}

func (srv *APIServer) status(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]string{"status": "OK"})
}
