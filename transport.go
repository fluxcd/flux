package flux

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/go-kit/kit/log"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"golang.org/x/net/context"
)

var (
	// ErrServiceRequired indicates a request could not be served because the
	// service parameter was required but not specified.
	ErrServiceRequired = errors.New("service parameter is required")
)

// MakeHTTPHandler mounts all of the service endpoints into an http.Handler.
// Useful in a server i.e. fluxd.
func MakeHTTPHandler(ctx context.Context, e Endpoints, logger log.Logger) http.Handler {
	r := mux.NewRouter().PathPrefix("/v0").Subrouter()
	options := []httptransport.ServerOption{
		httptransport.ServerErrorLogger(logger),
		httptransport.ServerErrorEncoder(encodeError),
	}

	r.Methods("GET").Path("/services/").Handler(httptransport.NewServer(
		ctx,
		e.ServicesEndpoint,
		decodeServicesRequest,
		encodeServicesResponse,
		options...,
	))
	r.Methods("POST").Path("/release/").Handler(httptransport.NewServer(
		ctx,
		e.ReleaseEndpoint,
		decodeReleaseRequest,
		encodeReleaseResponse,
		options...,
	))

	return r
}

func encodeJSON(_ context.Context, response interface{}, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(response)
}

func encodeError(_ context.Context, err error, w http.ResponseWriter) {
	if err == nil {
		panic("encodeError called with nil error")
	}
	code := codeFrom(err)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":       err.Error(),
		"status_code": code,
		"status_text": http.StatusText(code),
	})
}

func codeFrom(err error) int {
	switch err {
	case nil:
		panic("codeFrom called with nil error")
	case ErrServiceRequired:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func decodeServicesRequest(_ context.Context, r *http.Request) (request interface{}, err error) {
	namespace := r.FormValue("namespace")
	if namespace == "" {
		namespace = "default"
	}
	return servicesRequest{
		Namespace: namespace,
	}, nil
}

func decodeReleaseRequest(_ context.Context, r *http.Request) (request interface{}, err error) {
	namespace := r.FormValue("namespace")
	if namespace == "" {
		namespace = "default"
	}
	service := r.FormValue("service")
	if service == "" {
		return nil, ErrServiceRequired
	}
	newDef, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	updatePeriodStr := r.FormValue("updatePeriod")
	if updatePeriodStr == "" {
		updatePeriodStr = "5s"
	}
	updatePeriod, err := time.ParseDuration(updatePeriodStr)
	if err != nil {
		return nil, err
	}
	return releaseRequest{
		Namespace:    namespace,
		Service:      service,
		NewDef:       newDef,
		UpdatePeriod: updatePeriod,
	}, nil
}

func encodeServicesResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	resp := response.(servicesResponse)
	if resp.Err != nil {
		encodeError(ctx, resp.Err, w)
		return nil
	}
	encodeJSON(ctx, resp.Services, w)
	return nil
}

func encodeReleaseResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	resp := response.(releaseResponse)
	if resp.Err != nil {
		encodeError(ctx, resp.Err, w)
		return nil
	}
	encodeJSON(ctx, map[string]interface{}{"success": true}, w)
	return nil
}
