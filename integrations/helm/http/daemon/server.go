package daemon

import (
	"context"
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/weaveworks/flux/integrations/helm/api"
	transport "github.com/weaveworks/flux/integrations/helm/http"
	"net/http"
	"sync/atomic"
	"time"
)

// ListenAndServe starts a HTTP server instrumented with Prometheus metrics,
// health and API endpoints on the specified address.
func ListenAndServe(listenAddr string, apiServer api.Server, logger log.Logger, stopCh <-chan struct{}) {
	mux := http.DefaultServeMux

	// setup metrics and health endpoints
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// setup api endpoints
	handler := NewHandler(apiServer, transport.NewRouter())
	mux.Handle("/api/", http.StripPrefix("/api", handler))

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 1 * time.Minute,
		IdleTimeout:  15 * time.Second,
	}

	logger.Log("info", fmt.Sprintf("Starting HTTP server on %s", listenAddr))

	// run server in background
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			logger.Log("error", fmt.Sprintf("HTTP server crashed %v", err))
		}
	}()

	// wait for close signal and attempt graceful shutdown
	<-stopCh
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Log("warn", fmt.Sprintf("HTTP server graceful shutdown failed %v", err))
	} else {
		logger.Log("info", "HTTP server stopped")
	}
}

// NewHandler registers handlers on the given router.
func NewHandler(s api.Server, r *mux.Router) http.Handler {
	handle := &APIServer{server: s}
	r.Get(transport.SyncGit).HandlerFunc(handle.SyncGit)
	return r
}

type APIServer struct {
	server     api.Server
	syncingGit uint32
}

// SyncGit starts a goroutine in the background to sync all git mirrors
// _if there is not one running at time of request_. It writes back a
// HTTP 200 status header and 'OK' body to inform the request was
// successful.
// TODO(hidde): in the future we may want to give users the option to
// request the status after it has been started. The Flux (daemon) API
// achieves this by working with jobs whos IDs can be tracked.
func (s *APIServer) SyncGit(w http.ResponseWriter, r *http.Request) {
	if atomic.CompareAndSwapUint32(&s.syncingGit, 0, 1) {
		go func() {
			s.server.SyncMirrors()
			atomic.StoreUint32(&s.syncingGit, 0)
		}()
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
