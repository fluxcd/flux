package fluxsvc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	"github.com/weaveworks/fluxy/flux"
	fluxhttp "github.com/weaveworks/fluxy/flux/http"
	"github.com/weaveworks/fluxy/history"
)

// NewRouter returns a router with the fluxsvc routes.
func NewRouter() *mux.Router {
	r := mux.NewRouter()
	r.NewRoute().Name("ListServices").Methods("GET").Path("/v2/services").Queries("namespace", "{namespace}") // optional namespace!
	r.NewRoute().Name("ListImages").Methods("GET").Path("/v2/images").Queries("service", "{service}")
	r.NewRoute().Name("Release").Methods("POST").Path("/v2/release").Queries("service", "{service}", "image", "{image}", "kind", "{kind}")
	r.NewRoute().Name("Automate").Methods("POST").Path("/v2/automate").Queries("service", "{service}")
	r.NewRoute().Name("Deautomate").Methods("POST").Path("/v2/deautomate").Queries("service", "{service}")
	r.NewRoute().Name("History").Methods("GET").Path("/v2/history").Queries("service", "{service}")
	return r
}

// NewHandler returns an HTTP handler that can serve the fluxd routes.
func NewHandler(s Service, r *mux.Router, logger log.Logger, h metrics.Histogram) http.Handler {
	for method, handler := range map[string]http.Handler{
		"ListServices": handleListServices(s),
		"ListImages":   handleListImages(s),
		"Release":      handleRelease(s),
		"Automate":     handleAutomate(s),
		"Deautomate":   handleDeautomate(s),
		"History":      handleHistory(s),
	} {
		handler = fluxhttp.Logging(handler, log.NewContext(logger).With("method", method))
		handler = fluxhttp.Observing(handler, h.With("method", method))
		r.Get(method).Handler(handler)
	}
	return r
}

// The idea here is to place the handleFoo and invokeFoo functions next to each
// other, so changes in one can easily be accommodated in the other.

func handleListServices(sl ServiceLister) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgID := r.Header.Get(fluxhttp.OrgIDHeaderKey)
		namespace := mux.Vars(r)["namespace"]
		res, err := sl.ListServices(orgID, namespace)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(res); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}
	})
}

func invokeListServices(client *http.Client, router *mux.Router, endpoint string, namespace string) ([]flux.ServiceStatus, error) {
	u, err := fluxhttp.MakeURL(endpoint, router, "ListServices", "namespace", namespace)
	if err != nil {
		return nil, errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, errors.Wrapf(err, "constructing request %s", u)
	}

	resp, err := fluxhttp.ExecuteRequest(client, req)
	if err != nil {
		return nil, errors.Wrap(err, "executing HTTP request")
	}

	var res []flux.ServiceStatus
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, errors.Wrap(err, "decoding response from server")
	}
	return res, nil
}

func handleListImages(il ImageLister) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgID := r.Header.Get(fluxhttp.OrgIDHeaderKey)
		service := mux.Vars(r)["service"]
		spec, err := flux.ParseServiceSpec(service)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, errors.Wrapf(err, "parsing service spec %q", service).Error())
			return
		}
		d, err := il.ListImages(orgID, spec)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(d); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}
	})
}

func invokeListImages(client *http.Client, router *mux.Router, endpoint string, s flux.ServiceSpec) ([]flux.ImageStatus, error) {
	u, err := fluxhttp.MakeURL(endpoint, router, "ListImages", "service", string(s))
	if err != nil {
		return nil, errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, errors.Wrapf(err, "constructing request %s", u)
	}

	resp, err := fluxhttp.ExecuteRequest(client, req)
	if err != nil {
		return nil, errors.Wrap(err, "executing HTTP request")
	}

	var res []flux.ImageStatus
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, errors.Wrap(err, "decoding response from server")
	}
	return res, nil
}

func handleRelease(rel Releaser) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			orgID   = r.Header.Get(fluxhttp.OrgIDHeaderKey)
			vars    = mux.Vars(r)
			service = vars["service"]
			image   = vars["image"]
			kind    = vars["kind"]
		)
		serviceSpec, err := flux.ParseServiceSpec(service)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, errors.Wrapf(err, "parsing service spec %q", service).Error())
			return
		}
		imageSpec := flux.ParseImageSpec(image)
		releaseKind, err := flux.ParseReleaseKind(kind)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, errors.Wrapf(err, "parsing release kind %q", kind).Error())
			return
		}

		a, err := rel.Release(orgID, serviceSpec, imageSpec, releaseKind)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(a); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}
	})
}

func invokeRelease(client *http.Client, router *mux.Router, endpoint string, s flux.ServiceSpec, i flux.ImageSpec, k flux.ReleaseKind) ([]flux.ReleaseAction, error) {
	u, err := fluxhttp.MakeURL(endpoint, router, "Release", "service", string(s), "image", string(i), "kind", string(k))
	if err != nil {
		return nil, errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		return nil, errors.Wrapf(err, "constructing request %s", u)
	}

	resp, err := fluxhttp.ExecuteRequest(client, req)
	if err != nil {
		return nil, errors.Wrap(err, "executing HTTP request")
	}

	var res []flux.ReleaseAction
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, errors.Wrap(err, "decoding response from server")
	}
	return res, nil
}

func handleAutomate(a Automator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgID := r.Header.Get(fluxhttp.OrgIDHeaderKey)
		service := mux.Vars(r)["service"]
		serviceID, err := flux.ParseServiceID(service)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, errors.Wrapf(err, "parsing service ID %q", serviceID).Error())
			return
		}

		if err = a.Automate(orgID, serviceID); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}

func invokeAutomate(client *http.Client, router *mux.Router, endpoint string, s flux.ServiceID) error {
	u, err := fluxhttp.MakeURL(endpoint, router, "Automate", "service", string(s))
	if err != nil {
		return errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		return errors.Wrapf(err, "constructing request %s", u)
	}

	if _, err = fluxhttp.ExecuteRequest(client, req); err != nil {
		return errors.Wrap(err, "executing HTTP request")
	}

	return nil
}

func handleDeautomate(a Automator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgID := r.Header.Get(fluxhttp.OrgIDHeaderKey)
		service := mux.Vars(r)["service"]
		serviceID, err := flux.ParseServiceID(service)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, errors.Wrapf(err, "parsing service ID %q", serviceID).Error())
			return
		}

		if err = a.Deautomate(orgID, serviceID); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}

func invokeDeautomate(client *http.Client, router *mux.Router, endpoint string, id flux.ServiceID) error {
	u, err := fluxhttp.MakeURL(endpoint, router, "Deautomate", "service", string(id))
	if err != nil {
		return errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		return errors.Wrapf(err, "constructing request %s", u)
	}

	if _, err = fluxhttp.ExecuteRequest(client, req); err != nil {
		return errors.Wrap(err, "executing HTTP request")
	}

	return nil
}

func handleHistory(h HistoryReader) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgID := r.Header.Get(fluxhttp.OrgIDHeaderKey)
		service := mux.Vars(r)["service"]
		nstr := mux.Vars(r)["n"]
		n, err := strconv.ParseInt(nstr, 10, 64)
		if err != nil {
			n = 0
		}
		spec, err := flux.ParseServiceSpec(service)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, errors.Wrapf(err, "parsing service spec %q", spec).Error())
			return
		}

		h, err := h.History(orgID, spec, int(n))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(h); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}
	})
}

func invokeHistory(client *http.Client, router *mux.Router, endpoint string, s flux.ServiceSpec) ([]history.Event, error) {
	u, err := fluxhttp.MakeURL(endpoint, router, "History", "service", string(s))
	if err != nil {
		return nil, errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, errors.Wrapf(err, "constructing request %s", u)
	}

	resp, err := fluxhttp.ExecuteRequest(client, req)
	if err != nil {
		return nil, errors.Wrap(err, "executing HTTP request")
	}

	var res []history.Event
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, errors.Wrap(err, "decoding response from server")
	}

	return res, nil
}
