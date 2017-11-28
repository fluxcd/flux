package registry

import (
	"net/http"
	"net/url"
	"sync"

	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/auth/challenge"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/registry/middleware"
)

type RemoteClientFactory struct {
	Logger           log.Logger
	Limiters         *middleware.RateLimiters
	Trace            bool
	challengeManager challenge.Manager
	mx               sync.Mutex
}

type logging struct {
	logger    log.Logger
	transport http.RoundTripper
}

func (t *logging) RoundTrip(req *http.Request) (*http.Response, error) {
	res, err := t.transport.RoundTrip(req)
	if err == nil {
		t.logger.Log("url", req.URL.String(), "status", res.Status)
	} else {
		t.logger.Log("url", req.URL.String(), "err", err.Error())
	}
	return res, err
}

func (f *RemoteClientFactory) ClientFor(repo image.CanonicalName, creds Credentials) (Client, error) {
	tx := f.Limiters.RoundTripper(http.DefaultTransport, repo.Domain)
	if f.Trace {
		tx = &logging{f.Logger, tx}
	}

	f.mx.Lock()
	if f.challengeManager == nil {
		f.challengeManager = challenge.NewSimpleManager()
	}
	f.mx.Unlock()
	manager := f.challengeManager

	pingURL := url.URL{
		Scheme: "https",
		Host:   repo.Domain,
		Path:   "/v2/",
	}
	// Before we know how to authorise, need to establish which
	// authorisation challenges the host will send.
	if cs, err := manager.GetChallenges(pingURL); err == nil {
		if len(cs) == 0 {
			req, err := http.NewRequest("GET", pingURL.String(), nil)
			if err != nil {
				return nil, err
			}
			res, err := (&http.Client{
				Transport: tx,
			}).Do(req)
			if err != nil {
				return nil, err
			}
			if err = manager.AddResponse(res); err != nil {
				return nil, err
			}
		}
	}

	handler := auth.NewTokenHandler(tx, &store{creds}, repo.Image, "pull")
	tx = transport.NewTransport(tx, auth.NewAuthorizer(manager, handler))

	client := &Remote{transport: tx, repo: repo}
	return NewInstrumentedClient(client), nil
}

// credentialStore adapts our Credentials type to be an
// auth.CredentialsStore
type store struct {
	creds Credentials
}

func (s *store) Basic(url *url.URL) (string, string) {
	auth := s.creds.credsFor(url.Host)
	return auth.username, auth.password
}

func (s *store) RefreshToken(*url.URL, string) string {
	return ""
}

func (s *store) SetRefreshToken(*url.URL, string, string) {
	return
}
