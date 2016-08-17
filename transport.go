package flux

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

func NewRouter() *mux.Router {
	r := mux.NewRouter()
	r.NewRoute().Name("ListServices").Methods("GET").Path("/v1/services")
	r.NewRoute().Name("ListImages").Methods("GET").Path("/v1/images").Queries("service", "{service}")
	r.NewRoute().Name("Release").Methods("POST").Path("/v1/release").Queries("service", "{service}", "image", "{image}", "kind", "{kind}")
	r.NewRoute().Name("Automate").Methods("POST").Path("/v1/automate").Queries("service", "{service}")
	r.NewRoute().Name("Deautomate").Methods("POST").Path("/v1/deautomate").Queries("service", "{service}")
	r.NewRoute().Name("History").Methods("GET").Path("/v1/history").Queries("service", "{service}")
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

// The idea here is to place the handleFoo and invokeFoo functions next to each
// other, so changes in one can easily be accommodated in the other.

func handleListServices(s Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d, err := s.ListServices()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(d)
	})
}

func invokeListServices(client *http.Client, router *mux.Router, endpoint string) ([]ServiceDescription, error) {
	u, err := makeURL(endpoint, router, "ListServices")
	if err != nil {
		return nil, errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, errors.Wrapf(err, "constructing request %s", u)
	}

	resp, err := executeRequest(client, req)
	if err != nil {
		return nil, errors.Wrap(err, "executing HTTP request")
	}

	var res []ServiceDescription
	if err := json.NewDecoder(resp.Body).Decode(res); err != nil {
		return nil, errors.Wrap(err, "decoding response from server")
	}
	return res, nil
}

func handleListImages(s Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		service := mux.Vars(r)["service"]
		spec, err := ParseServiceSpec(service)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, errors.Wrapf(err, "parsing service spec %q", service).Error())
			return
		}
		d, err := s.ListImages(spec)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(d)
	})
}

func invokeListImages(client *http.Client, router *mux.Router, endpoint string, s ServiceSpec) ([]ImageDescription, error) {
	u, err := makeURL(endpoint, router, "ListImages", "service", string(s))
	if err != nil {
		return nil, errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, errors.Wrapf(err, "constructing request %s", u)
	}

	resp, err := executeRequest(client, req)
	if err != nil {
		return nil, errors.Wrap(err, "executing HTTP request")
	}

	var res []ImageDescription
	if err := json.NewDecoder(resp.Body).Decode(res); err != nil {
		return nil, errors.Wrap(err, "decoding response from server")
	}
	return res, nil
}

func handleRelease(s Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			vars    = mux.Vars(r)
			service = vars["service"]
			image   = vars["image"]
			kind    = vars["kind"]
		)
		serviceSpec, err := ParseServiceSpec(service)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, errors.Wrapf(err, "parsing service spec %q", service).Error())
			return
		}
		imageSpec := ParseImageSpec(image)
		releaseKind, err := ParseReleaseKind(kind)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, errors.Wrapf(err, "parsing release kind %q", kind).Error())
			return
		}

		a, err := s.Release(serviceSpec, imageSpec, releaseKind)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(a)
	})
}

func invokeRelease(client *http.Client, router *mux.Router, endpoint string, s ServiceSpec, i ImageSpec, k ReleaseKind) ([]ReleaseAction, error) {
	u, err := makeURL(endpoint, router, "Release", "service", string(s), "image", string(i), "kind", string(k))
	if err != nil {
		return nil, errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		return nil, errors.Wrapf(err, "constructing request %s", u)
	}

	resp, err := executeRequest(client, req)
	if err != nil {
		return nil, errors.Wrap(err, "executing HTTP request")
	}

	var res []ReleaseAction
	if err := json.NewDecoder(resp.Body).Decode(res); err != nil {
		return nil, errors.Wrap(err, "decoding response from server")
	}
	return res, nil
}

func handleAutomate(s Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		service := mux.Vars(r)["service"]
		id, err := ParseServiceID(service)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, errors.Wrapf(err, "parsing service ID %q", id).Error())
			return
		}

		if err = s.Automate(id); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}

func invokeAutomate(client *http.Client, router *mux.Router, endpoint string, s ServiceID) error {
	u, err := makeURL(endpoint, router, "Automate", "service", string(s))
	if err != nil {
		return errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		return errors.Wrapf(err, "constructing request %s", u)
	}

	if _, err = executeRequest(client, req); err != nil {
		return errors.Wrap(err, "executing HTTP request")
	}

	return nil
}

func handleDeautomate(s Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		service := mux.Vars(r)["service"]
		id, err := ParseServiceID(service)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, errors.Wrapf(err, "parsing service ID %q", id).Error())
			return
		}

		if err = s.Deautomate(id); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}

func invokeDeautomate(client *http.Client, router *mux.Router, endpoint string, id ServiceID) error {
	u, err := makeURL(endpoint, router, "Deautomate", "service", string(id))
	if err != nil {
		return errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		return errors.Wrapf(err, "constructing request %s", u)
	}

	if _, err = executeRequest(client, req); err != nil {
		return errors.Wrap(err, "executing HTTP request")
	}

	return nil
}

func handleHistory(s Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		service := mux.Vars(r)["service"]
		spec, err := ParseServiceSpec(service)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, errors.Wrapf(err, "parsing service spec %q", spec).Error())
			return
		}

		h, err := s.History(spec)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(h)
	})
}

func invokeHistory(client *http.Client, router *mux.Router, endpoint string, s ServiceSpec) ([]HistoryEntry, error) {
	u, err := makeURL(endpoint, router, "History", "service", string(s))
	if err != nil {
		return nil, errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, errors.Wrapf(err, "constructing request %s", u)
	}

	resp, err := executeRequest(client, req)
	if err != nil {
		return nil, errors.Wrap(err, "executing HTTP request")
	}

	var res []HistoryEntry
	if err := json.NewDecoder(resp.Body).Decode(res); err != nil {
		return nil, errors.Wrap(err, "decoding response from server")
	}

	return res, nil
}

func mustGetPathTemplate(route *mux.Route) string {
	t, err := route.GetPathTemplate()
	if err != nil {
		panic(err)
	}
	return t
}

func makeURL(endpoint string, router *mux.Router, routeName string, urlParams ...string) (*url.URL, error) {
	if len(urlParams)%2 != 0 {
		panic("urlParams must be even!")
	}

	endpointURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing endpoint %s", endpoint)
	}

	routeURL, err := router.Get(routeName).URL()
	if err != nil {
		return nil, errors.Wrapf(err, "retrieving route path %s", routeName)
	}

	v := url.Values{}
	for i := 0; i < len(urlParams); i += 2 {
		v.Set(urlParams[i], urlParams[i+1])
	}

	endpointURL.Path = routeURL.Path
	endpointURL.RawQuery = v.Encode()
	return endpointURL, nil
}

func executeRequest(client *http.Client, req *http.Request) (*http.Response, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "executing HTTP request")
	}
	if resp.StatusCode != http.StatusOK {
		buf, _ := ioutil.ReadAll(resp.Body)
		err = fmt.Errorf("%s (%s)", resp.Status, strings.TrimSpace(string(buf)))
		return nil, errors.Wrap(err, "reading HTTP response")
	}
	return resp, nil
}
