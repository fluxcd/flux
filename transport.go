package flux

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/go-kit/kit/log"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"golang.org/x/net/context"
)

var (
	// ErrRepositoryRequired indicates a request could not be served because the
	// repository parameter was required but not specified.
	ErrRepositoryRequired = errors.New("repository (as part of the URL path) is required")

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

	r.Methods("GET").Path("/images/{repository:.*}").Handler(httptransport.NewServer(
		ctx,
		e.ImagesEndpoint,
		decodeImagesRequest,
		encodeImagesResponse,
		options...,
	))
	r.Methods("GET").Path("/services").Handler(httptransport.NewServer(
		ctx,
		e.ServicesEndpoint,
		decodeServicesRequest,
		encodeServicesResponse,
		options...,
	))
	r.Methods("GET").Path("/service/{service}/images").Handler(httptransport.NewServer(
		ctx,
		e.ServiceImagesEndpoint,
		decodeServiceImagesRequest,
		encodeServiceImagesResponse,
		options...,
	))
	r.Methods("GET").Path("/history/{service:.*}").Handler(httptransport.NewServer(
		ctx,
		e.HistoryEndpoint,
		decodeHistoryRequest,
		encodeHistoryResponse,
		options...,
	))
	r.Methods("POST").Path("/release").Handler(httptransport.NewServer(
		ctx,
		e.ReleaseEndpoint,
		decodeReleaseRequest,
		encodeReleaseResponse,
		options...,
	))
	r.Methods("POST").Path("/automate").Handler(httptransport.NewServer(
		ctx,
		e.AutomateEndpoint,
		decodeAutomateRequest,
		encodeAutomateResponse,
		options...,
	))
	r.Methods("POST").Path("/deautomate").Handler(httptransport.NewServer(
		ctx,
		e.DeautomateEndpoint,
		decodeDeautomateRequest,
		encodeDeautomateResponse,
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

func decodeError(r io.Reader) error {
	var m map[string]interface{}
	if err := json.NewDecoder(r).Decode(&m); err != nil {
		return fmt.Errorf("received error, but encountered additional error when trying to parse it: %v", err)
	}
	err, ok := m["error"]
	if !ok {
		return errors.New("received error, but it was an unexpected form, so is unknown")
	}
	errStr, ok := err.(string)
	if !ok {
		return errors.New("received error, but it was an unexpected type, so is unknown")
	}
	return errors.New(errStr)
}

func codeFrom(err error) int {
	switch err {
	case nil:
		panic("codeFrom called with nil error")
	case ErrRepositoryRequired, ErrServiceRequired:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func decodeImagesRequest(_ context.Context, r *http.Request) (request interface{}, err error) {
	repository := mux.Vars(r)["repository"]
	if repository == "" {
		return nil, ErrRepositoryRequired
	}
	return imagesRequest{
		Repository: repository,
	}, nil
}

func decodeServiceImagesRequest(_ context.Context, r *http.Request) (request interface{}, err error) {
	namespace := r.FormValue("namespace")
	if namespace == "" {
		namespace = DefaultNamespace
	}
	service := mux.Vars(r)["service"]
	return serviceImagesRequest{
		Namespace: namespace,
		Service:   service,
	}, nil
}

func decodeServicesRequest(_ context.Context, r *http.Request) (request interface{}, err error) {
	namespace := r.FormValue("namespace")
	if namespace == "" {
		namespace = DefaultNamespace
	}
	return servicesRequest{
		Namespace: namespace,
	}, nil
}

func decodeHistoryRequest(_ context.Context, r *http.Request) (request interface{}, err error) {
	namespace := r.FormValue("namespace")
	if namespace == "" {
		namespace = DefaultNamespace
	}
	service := mux.Vars(r)["service"]
	return historyRequest{
		Namespace: namespace,
		Service:   service,
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
	image := r.FormValue("image")
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
		Image:        image,
		NewDef:       newDef,
		UpdatePeriod: updatePeriod,
	}, nil
}

func decodeAutomateRequest(_ context.Context, r *http.Request) (request interface{}, err error) {
	namespace := r.FormValue("namespace")
	if namespace == "" {
		namespace = "default"
	}
	service := r.FormValue("service")
	if service == "" {
		return nil, ErrServiceRequired
	}
	return automateRequest{
		Namespace: namespace,
		Service:   service,
	}, nil
}

func decodeDeautomateRequest(_ context.Context, r *http.Request) (request interface{}, err error) {
	namespace := r.FormValue("namespace")
	if namespace == "" {
		namespace = "default"
	}
	service := r.FormValue("service")
	if service == "" {
		return nil, ErrServiceRequired
	}
	return deautomateRequest{
		Namespace: namespace,
		Service:   service,
	}, nil
}

func decodeImagesResponse(_ context.Context, resp *http.Response) (interface{}, error) {
	var response imagesResponse
	var err error
	switch resp.StatusCode {
	case http.StatusOK:
		err = json.NewDecoder(resp.Body).Decode(&response.Images)
	default:
		response.Err = decodeError(resp.Body)
	}
	return response, err
}

func decodeServiceImagesResponse(_ context.Context, resp *http.Response) (interface{}, error) {
	var response serviceImagesResponse
	var err error
	switch resp.StatusCode {
	case http.StatusOK:
		err = json.NewDecoder(resp.Body).Decode(&response.ContainerImages)
	default:
		response.Err = decodeError(resp.Body)
	}
	return response, err
}

func decodeServicesResponse(_ context.Context, resp *http.Response) (interface{}, error) {
	var response servicesResponse
	var err error
	switch resp.StatusCode {
	case http.StatusOK:
		err = json.NewDecoder(resp.Body).Decode(&response.Services)
	default:
		response.Err = decodeError(resp.Body)
	}
	return response, err
}

func decodeHistoryResponse(_ context.Context, resp *http.Response) (interface{}, error) {
	var response historyResponse
	var err error
	switch resp.StatusCode {
	case http.StatusOK:
		err = json.NewDecoder(resp.Body).Decode(&response.History)
	default:
		response.Err = decodeError(resp.Body)
	}
	return response, err
}

func decodeReleaseResponse(_ context.Context, resp *http.Response) (interface{}, error) {
	var response releaseResponse
	switch resp.StatusCode {
	case http.StatusOK:
		// nothing to do
	default:
		response.Err = decodeError(resp.Body)
	}
	return response, nil
}

func decodeAutomateResponse(_ context.Context, resp *http.Response) (interface{}, error) {
	var response automateResponse
	switch resp.StatusCode {
	case http.StatusOK:
		// nothing to do
	default:
		response.Err = decodeError(resp.Body)
	}
	return response, nil
}

func decodeDeautomateResponse(_ context.Context, resp *http.Response) (interface{}, error) {
	var response deautomateResponse
	switch resp.StatusCode {
	case http.StatusOK:
		// nothing to do
	default:
		response.Err = decodeError(resp.Body)
	}
	return response, nil
}

func encodeImagesRequest(ctx context.Context, req *http.Request, request interface{}) error {
	r := request.(imagesRequest)

	req.Method = "GET"
	req.URL.Path = path.Join(req.URL.Path, "/v0/images/"+r.Repository)

	return nil
}

func encodeServiceImagesRequest(ctx context.Context, req *http.Request, request interface{}) error {
	r := request.(serviceImagesRequest)
	values := url.Values{}
	values.Set("namespace", r.Namespace)

	req.Method = "GET"
	req.URL.Path = path.Join(req.URL.Path, fmt.Sprintf("/v0/service/%s/images", r.Service))
	req.URL.RawQuery = values.Encode()

	return nil
}

func encodeServicesRequest(ctx context.Context, req *http.Request, request interface{}) error {
	r := request.(servicesRequest)
	values := url.Values{}
	values.Set("namespace", r.Namespace)

	req.Method = "GET"
	req.URL.Path = path.Join(req.URL.Path, "/v0/services")
	req.URL.RawQuery = values.Encode()

	return nil
}

func encodeHistoryRequest(ctx context.Context, req *http.Request, request interface{}) error {
	r := request.(historyRequest)
	values := url.Values{}
	values.Set("namespace", r.Namespace)

	req.Method = "GET"
	// NB be careful here: Join will strip a trailing slash
	req.URL.Path = fmt.Sprintf(path.Join(req.URL.Path, "/v0/history/%s"), r.Service)
	req.URL.RawQuery = values.Encode()

	return nil
}

func encodeReleaseRequest(ctx context.Context, req *http.Request, request interface{}) error {
	r := request.(releaseRequest)
	values := url.Values{}
	values.Set("namespace", r.Namespace)
	values.Set("service", r.Service)
	values.Set("image", r.Image)
	values.Set("updatePeriod", r.UpdatePeriod.String())

	req.Method = "POST"
	req.URL.Path = path.Join(req.URL.Path, "/v0/release")
	req.URL.RawQuery = values.Encode()
	req.Body = ioutil.NopCloser(bytes.NewReader(r.NewDef))

	return nil
}

func encodeAutomateRequest(ctx context.Context, req *http.Request, request interface{}) error {
	r := request.(automateRequest)
	values := url.Values{}
	values.Set("namespace", r.Namespace)
	values.Set("service", r.Service)

	req.Method = "POST"
	req.URL.Path = path.Join(req.URL.Path, "/v0/automate")
	req.URL.RawQuery = values.Encode()

	return nil
}

func encodeDeautomateRequest(ctx context.Context, req *http.Request, request interface{}) error {
	r := request.(deautomateRequest)
	values := url.Values{}
	values.Set("namespace", r.Namespace)
	values.Set("service", r.Service)

	req.Method = "POST"
	req.URL.Path = path.Join(req.URL.Path, "/v0/deautomate")
	req.URL.RawQuery = values.Encode()

	return nil
}

func encodeImagesResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	resp := response.(imagesResponse)
	if resp.Err != nil {
		encodeError(ctx, resp.Err, w)
		return nil
	}
	encodeJSON(ctx, resp.Images, w)
	return nil
}

func encodeServiceImagesResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	resp := response.(serviceImagesResponse)
	if resp.Err != nil {
		encodeError(ctx, resp.Err, w)
		return nil
	}
	encodeJSON(ctx, resp.ContainerImages, w)
	return nil
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

func encodeHistoryResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	resp := response.(historyResponse)
	if resp.Err != nil {
		encodeError(ctx, resp.Err, w)
		return nil
	}
	encodeJSON(ctx, resp.History, w)
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

func encodeAutomateResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	resp := response.(automateResponse)
	if resp.Err != nil {
		encodeError(ctx, resp.Err, w)
		return nil
	}
	encodeJSON(ctx, map[string]interface{}{"success": true}, w)
	return nil
}

func encodeDeautomateResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	resp := response.(deautomateResponse)
	if resp.Err != nil {
		encodeError(ctx, resp.Err, w)
		return nil
	}
	encodeJSON(ctx, map[string]interface{}{"success": true}, w)
	return nil
}
