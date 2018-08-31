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
	Logger        log.Logger
	Limiters      *middleware.RateLimiters
	Trace         bool
	InsecureHosts []string

	mu               sync.Mutex
	challengeManager challenge.Manager
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

	f.mu.Lock()
	if f.challengeManager == nil {
		f.challengeManager = challenge.NewSimpleManager()
	}
	manager := f.challengeManager
	f.mu.Unlock()

	scheme := "https"
	for _, h := range f.InsecureHosts {
		if repo.Domain == h {
			scheme = "http"
		}
	}

	registryURL := url.URL{
		Scheme: scheme,
		Host:   repo.Domain,
		Path:   "/v2/",
	}

	// Before we know how to authorise, need to establish which
	// authorisation challenges the host will send. See if we've been
	// here before.
	cs, err := manager.GetChallenges(registryURL)
	if err != nil {
		return nil, err
	}
	if len(cs) == 0 {
		// No prior challenge; try pinging the registry endpoint to
		// get a challenge. `http.Client` will follow redirects, so
		// even if we thought it was an insecure (HTTP) host, we may
		// end up requesting HTTPS.
		req, err := http.NewRequest("GET", registryURL.String(), nil)
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
		registryURL = *res.Request.URL // <- the URL after any redirection
	}

	cred := creds.credsFor(repo.Domain)
	if f.Trace {
		f.Logger.Log("repo", repo.String(), "auth", cred.String(), "api", registryURL.String())
	}

	tokenHandler := auth.NewTokenHandler(tx, &store{cred}, repo.Image, "pull")
	basicauthHandler := auth.NewBasicHandler(&store{cred})
	tx = transport.NewTransport(tx, auth.NewAuthorizer(manager, tokenHandler, basicauthHandler))

	// For the API base we want only the scheme and host.
	registryURL.Path = ""
	client := &Remote{transport: tx, repo: repo, base: registryURL.String()}
	return NewInstrumentedClient(client), nil
}

// Succeed exists merely so that the user of the ClientFactory can
// bump rate limits up if a repo's metadata has successfully been
// fetched.
func (f *RemoteClientFactory) Succeed(repo image.CanonicalName) {
	f.Limiters.Recover(repo.Domain)
}

// store adapts a set of pre-selected creds to be an
// auth.CredentialsStore
type store struct {
	auth creds
}

func (s *store) Basic(url *url.URL) (string, string) {
	return s.auth.username, s.auth.password
}

func (s *store) RefreshToken(*url.URL, string) string {
	return ""
}

func (s *store) SetRefreshToken(*url.URL, string, string) {
	return
}
