package flux

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

func NewRouter() *mux.Router {
	r := mux.NewRouter()
	r.NewRoute().Name("ListServices").Methods("GET").Path("/v2/services").Queries("namespace", "{namespace}") // optional namespace!
	r.NewRoute().Name("ListImages").Methods("GET").Path("/v2/images").Queries("service", "{service}")
	r.NewRoute().Name("PostRelease").Methods("POST").Path("/v2/release").Queries("service", "{service}", "image", "{image}", "kind", "{kind}")
	r.NewRoute().Name("GetRelease").Methods("GET").Path("/v2/release").Queries("id", "{id}")
	r.NewRoute().Name("Automate").Methods("POST").Path("/v2/automate").Queries("service", "{service}")
	r.NewRoute().Name("Deautomate").Methods("POST").Path("/v2/deautomate").Queries("service", "{service}")
	r.NewRoute().Name("History").Methods("GET").Path("/v2/history").Queries("service", "{service}")
	return r
}

func NewHandler(s Service, r *mux.Router, logger log.Logger, h metrics.Histogram) http.Handler {
	for method, handlerFunc := range map[string]func(Service) http.Handler{
		"ListServices": handleListServices,
		"ListImages":   handleListImages,
		"PostRelease":  handlePostRelease,
		"GetRelease":   handleGetRelease,
		"Automate":     handleAutomate,
		"Deautomate":   handleDeautomate,
		"History":      handleHistory,
	} {
		var handler http.Handler
		handler = handlerFunc(s)
		handler = logging(handler, log.NewContext(logger).With("method", method))
		handler = observing(handler, h.With("method", method))

		r.Get(method).Handler(handler)
	}
	return r
}

// The idea here is to place the handleFoo and invokeFoo functions next to each
// other, so changes in one can easily be accommodated in the other.

func handleListServices(s Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		namespace := mux.Vars(r)["namespace"]
		res, err := s.ListServices(namespace)
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

func invokeListServices(client *http.Client, router *mux.Router, endpoint string, namespace string) ([]ServiceStatus, error) {
	u, err := makeURL(endpoint, router, "ListServices", "namespace", namespace)
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

	var res []ServiceStatus
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
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
		if err := json.NewEncoder(w).Encode(d); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}
	})
}

func invokeListImages(client *http.Client, router *mux.Router, endpoint string, s ServiceSpec) ([]ImageStatus, error) {
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

	var res []ImageStatus
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, errors.Wrap(err, "decoding response from server")
	}
	return res, nil
}

type postReleaseResponse struct {
	Status    string    `json:"status"`
	ReleaseID ReleaseID `json:"release_id"`
}

func handlePostRelease(s Service) http.Handler {
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

		var excludes []ServiceID
		for _, ex := range r.URL.Query()["exclude"] {
			s, err := ParseServiceID(ex)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, errors.Wrapf(err, "parsing excluded service %q", ex).Error())
				return
			}
			excludes = append(excludes, s)
		}

		id, err := s.PostRelease(ReleaseJobSpec{
			ServiceSpec: serviceSpec,
			ImageSpec:   imageSpec,
			Kind:        releaseKind,
			Excludes:    excludes,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(postReleaseResponse{
			Status:    "Submitted.",
			ReleaseID: id,
		}); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}
	})
}

func invokePostRelease(client *http.Client, router *mux.Router, endpoint string, s ReleaseJobSpec) (ReleaseID, error) {
	args := []string{"service", string(s.ServiceSpec), "image", string(s.ImageSpec), "kind", string(s.Kind)}
	for _, ex := range s.Excludes {
		args = append(args, "exclude", string(ex))
	}

	u, err := makeURL(endpoint, router, "PostRelease", args...)
	if err != nil {
		return "", errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		return "", errors.Wrapf(err, "constructing request %s", u)
	}

	resp, err := executeRequest(client, req)
	if err != nil {
		return "", errors.Wrap(err, "executing HTTP request")
	}

	var res postReleaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", errors.Wrap(err, "decoding response from server")
	}
	return res.ReleaseID, nil
}

func handleGetRelease(s Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := mux.Vars(r)["id"]
		job, err := s.GetRelease(ReleaseID(id))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(job); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}
	})
}

func invokeGetRelease(client *http.Client, router *mux.Router, endpoint string, id ReleaseID) (ReleaseJob, error) {
	u, err := makeURL(endpoint, router, "GetRelease", "id", string(id))
	if err != nil {
		return ReleaseJob{}, errors.Wrap(err, "constructing URL")
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return ReleaseJob{}, errors.Wrapf(err, "constructing request %s", u)
	}

	resp, err := executeRequest(client, req)
	if err != nil {
		return ReleaseJob{}, errors.Wrap(err, "executing HTTP request")
	}

	var res ReleaseJob
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return ReleaseJob{}, errors.Wrap(err, "decoding response from server")
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
		if err := json.NewEncoder(w).Encode(h); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}
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
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
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

	endpointURL.Path = path.Join(endpointURL.Path, routeURL.Path)
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

func observing(next http.Handler, h metrics.Histogram) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		begin := time.Now()
		cw := &codeWriter{w, http.StatusOK}
		next.ServeHTTP(cw, r)
		h.With("status_code", strconv.Itoa(cw.code)).Observe(time.Since(begin).Seconds())
	})
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

// teeWriter intercepts and stores the HTTP response.
type teeWriter struct {
	http.ResponseWriter
	buf bytes.Buffer
}

func (w *teeWriter) Write(p []byte) (int, error) {
	w.buf.Write(p) // best-effort
	return w.ResponseWriter.Write(p)
}

func mustUnescape(s string) string {
	if unescaped, err := url.QueryUnescape(s); err == nil {
		return unescaped
	}
	return s
}
