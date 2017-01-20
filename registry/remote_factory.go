// We can't inject a remote client directly because each repository request might be to a different
// registry provider. E.g. both docker hub and quay containers. So a new remote client must be
// created for each new Image. This factory provides that and can be mocked out.
package registry

import (
	"context"
	"github.com/go-kit/kit/log"
	dockerregistry "github.com/heroku/docker-registry-client/registry"
	"github.com/jonboulle/clockwork"
	"golang.org/x/net/publicsuffix"
	"net/http"
	"net/http/cookiejar"
	"time"
)

type creds struct {
	username, password string
}

// Credentials to a (Docker) registry.
type Credentials struct {
	m map[string]creds
}

type RemoteClientFactory interface {
	CreateFor(host string) (Remote, error)
}

func NewRemoteClientFactory(c Credentials, l log.Logger, mc MemcacheClient, ce time.Duration) RemoteClientFactory {
	return &remoteClientFactory{
		creds:          c,
		Logger:         l,
		MemcacheClient: mc,
		CacheExpiry:    ce,
	}
}

type remoteClientFactory struct {
	creds          Credentials
	Logger         log.Logger
	MemcacheClient MemcacheClient
	CacheExpiry    time.Duration
}

func (f *remoteClientFactory) CreateFor(host string) (_ Remote, err error) {
	client, cancel, err := f.newRegistryClient(host)
	if err != nil {
		return
	}
	return newRemote(client, cancel), nil
}

func (f *remoteClientFactory) newRegistryClient(host string) (client dockerRegistryInterface, cancel context.CancelFunc, err error) {
	httphost := "https://" + host

	// quay.io wants us to use cookies for authorisation, so we have
	// to construct one (the default client has none). This means a
	// bit more constructing things to be able to make a registry
	// client literal, rather than calling .New()
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return
	}
	auth := f.creds.credsFor(host)

	// A context we'll use to cancel requests on error
	ctx, cancel := context.WithCancel(context.Background())
	// Add a timeout to the request
	ctx, cancel = context.WithTimeout(ctx, requestTimeout)

	// Use the wrapper to fix headers for quay.io, and remember bearer tokens
	var transport http.RoundTripper = &wwwAuthenticateFixer{transport: http.DefaultTransport}
	// Now the auth-handling wrappers that come with the library
	transport = dockerregistry.WrapTransport(transport, httphost, auth.username, auth.password)
	// Add the backoff mechanism so we don't DOS registries
	transport = BackoffRoundTripper(transport, initialBackoff, maxBackoff, clockwork.NewRealClock())

	client = herokuWrapper{
		&dockerregistry.Registry{
			URL: httphost,
			Client: &http.Client{
				Transport: roundtripperFunc(func(r *http.Request) (*http.Response, error) {
					return transport.RoundTrip(r.WithContext(ctx))
				}),
				Jar:     jar,
				Timeout: requestTimeout,
			},
			Logf: dockerregistry.Quiet,
		},
	}
	if f.MemcacheClient != nil {
		client = NewCache(f.creds, f.MemcacheClient, f.CacheExpiry, f.Logger)(client)
	} else {
		f.Logger.Log("registry_cache", "disabled")
	}
	return
}

type roundtripperFunc func(*http.Request) (*http.Response, error)

func (f roundtripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
