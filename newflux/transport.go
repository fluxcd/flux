package flux

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
)

func NewRouter() *mux.Router {
	r := mux.NewRouter()
	r.NewRoute().Name("ListServices").Methods("GET").Path("/v0/services")
	r.NewRoute().Name("ListImages").Methods("GET").Path("/v0/images").Queries("service", "{service}")
	r.NewRoute().Name("Release").Methods("POST").Path("/v0/release").Queries("service", "{service}", "image", "{image}")
	r.NewRoute().Name("Automate").Methods("POST").Path("/v0/automate").Queries("service", "{service}")
	r.NewRoute().Name("Deautomate").Methods("POST").Path("/v0/deautomate").Queries("service", "{service}")
	r.NewRoute().Name("History").Methods("GET").Path("/v0/history")
	return r
}

func NewHandler(s Service, r *mux.Router) http.Handler {
	r.Get("ListServices").Handler(handleListServices(s))
	r.Get("ListImages").Handler(handleListImages(s))
	r.Get("Release").Handler(handleRelease(s))
	r.Get("Automate").Handler(handleAutomate(s))
	r.Get("Deautomate").Handler(handleDeautomate(s))
	r.Get("History").Handler(handleHistory(s))
	return r
}

// Arrange the handleFoo and invokeFoo functions next to each other, so changes
// in one can easily be accommodated in the other.

func handleListServices(s Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		fmt.Fprintf(w, "not implemented\n")
	})
}

func invokeListServices(client *http.Client, router *mux.Router, endpoint string) ([]ServiceDescription, error) {
	baseURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	pathURL, err := router.Get("ListServices").URLPath()
	if err != nil {
		return nil, err
	}
	baseURL.Path = pathURL.Path
	req, err := http.NewRequest("GET", baseURL.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	var res []ServiceDescription
	if err := json.NewDecoder(resp.Body).Decode(res); err != nil {
		return nil, err
	}
	return res, nil
}

func handleListImages(s Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		fmt.Fprintf(w, "not implemented\n")
	})
}

func invokeListImages(client *http.Client, router *mux.Router, endpoint string) ([]ImageDescription, error) {
	return nil, errors.New("not implemented")
}

func handleRelease(s Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		fmt.Fprintf(w, "not implemented\n")
	})
}

func invokeRelease(client *http.Client, router *mux.Router, endpoint string) error {
	return errors.New("not implemented")
}

func handleAutomate(s Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		fmt.Fprintf(w, "not implemented\n")
	})
}

func invokeAutomate(client *http.Client, router *mux.Router, endpoint string) error {
	return errors.New("not implemented")
}

func handleDeautomate(s Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		fmt.Fprintf(w, "not implemented\n")
	})
}

func invokeDeautomate(client *http.Client, router *mux.Router, endpoint string) error {
	return errors.New("not implemented")
}

func handleHistory(s Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		fmt.Fprintf(w, "not implemented\n")
	})
}

func invokeHistory(client *http.Client, router *mux.Router, endpoint string) ([]HistoryEntry, error) {
	return nil, errors.New("not implemented")
}

func mustGetPathTemplate(route *mux.Route) string {
	t, err := route.GetPathTemplate()
	if err != nil {
		panic(err)
	}
	return t
}

func mustURL(route *mux.Route) *url.URL {
	u, err := route.URL()
	if err != nil {
		panic(err)
	}
	return u
}
