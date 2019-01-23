package registry

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/auth/challenge"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/registry/middleware"
)

type RemoteClientFactory struct {
	Logger   log.Logger
	Limiters *middleware.RateLimiters
	Trace    bool
	// hosts with which to tolerate insecure connections (e.g., with
	// TLS_INSECURE_SKIP_VERIFY, or as a fallback, using HTTP).
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
	insecure := false
	for _, h := range f.InsecureHosts {
		if repo.Domain == h {
			insecure = true
			break
		}
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: insecure,
	}

	// Since we construct one of these per scan, be fairly ruthless
	// about throttling the number, and closing of, idle connections.
	baseTx := &http.Transport{
		TLSClientConfig: tlsConfig,
		MaxIdleConns:    10,
		IdleConnTimeout: 10 * time.Second,
		Proxy:           http.ProxyFromEnvironment,
	}
	tx := f.Limiters.RoundTripper(baseTx, repo.Domain)
	if f.Trace {
		tx = &logging{f.Logger, tx}
	}

	f.mu.Lock()
	if f.challengeManager == nil {
		f.challengeManager = challenge.NewSimpleManager()
	}
	manager := f.challengeManager
	f.mu.Unlock()

	registryURL := url.URL{
		Scheme: "https",
		Host:   repo.Domain,
		Path:   "/v2/",
	}

	// Before we know how to authorise, need to establish which
	// authorisation challenges the host will send. See if we've been
	// here before.
	attemptInsecureFallback := insecure
attempt:
	cs, err := manager.GetChallenges(registryURL)
	if err != nil {
		if attemptInsecureFallback {
			registryURL.Scheme = "http"
			attemptInsecureFallback = false
			goto attempt
		}
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
		ctx, cancel := context.WithTimeout(req.Context(), 30*time.Second)
		defer cancel()
		res, err := (&http.Client{
			Transport: tx,
		}).Do(req.WithContext(ctx))
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
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
