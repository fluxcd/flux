package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

// Server implements the fluxd API as consumed by the fluxctl client.
type Server struct{}

// ListenAndServe mirrors http.Server and should be invoked in func main.
func (s *Server) ListenAndServe(addr string) error {
	r := mux.NewRouter()
	s.installRoutes(r)
	return http.ListenAndServe(addr, r)
}

func (s *Server) installRoutes(r *mux.Router) {
	r.HandleFunc("/", s.status)
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]string{"status": "OK"})
}
