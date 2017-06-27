// We can't inject a remote client directly because each repository request might be to a different
// registry provider. E.g. both docker hub and quay containers. So a new remote client must be
// created for each new Image. This factory provides that and can be mocked out.
package registry

import (
	"context"
	"errors"
	"github.com/go-kit/kit/log"
	dockerregistry "github.com/heroku/docker-registry-client/registry"
	"github.com/jonboulle/clockwork"
	"golang.org/x/net/publicsuffix"
	"net/http"
	"net/http/cookiejar"
	"time"
)

var (
	ErrNoMemcache = errors.New("no memecache")
)

type creds struct {
	username, password string
}

// Credentials to a (Docker) registry.
type Credentials struct {
	m map[string]creds
}

type ClientFactory interface {
	ClientFor(host string) (client dockerRegistryInterface, cancel context.CancelFunc, err error)
}

func NewCacheClientFactory(c Credentials, l log.Logger, mc MemcacheClient, ce time.Duration) ClientFactory {
	for host, creds := range c.m {
		l.Log("host", host, "username", creds.username)
	}
	return &cacheClientFactory{
		creds:          c,
		Logger:         l,
		MemcacheClient: mc,
		CacheExpiry:    ce,
	}
}

type cacheClientFactory struct {
	creds          Credentials
	Logger         log.Logger
	MemcacheClient MemcacheClient
	CacheExpiry    time.Duration
}

func (f *cacheClientFactory) ClientFor(host string) (dockerRegistryInterface, context.CancelFunc, error) {
	if f.MemcacheClient == nil {
		return nil, nil, ErrNoMemcache
	}
	client := NewCache(f.creds, f.MemcacheClient, f.CacheExpiry, f.Logger)
	return client, func() {}, nil
}

func NewRemoteClientFactory(c Credentials, l log.Logger, rlc RateLimiterConfig) ClientFactory {
	for host, creds := range c.m {
		l.Log("host", host, "username", creds.username)
	}
	return &remoteClientFactory{
		creds:  c,
		Logger: l,
		rlConf: rlc,
	}
}

type remoteClientFactory struct {
	creds  Credentials
	Logger log.Logger
	rlConf RateLimiterConfig
}

func (f *remoteClientFactory) ClientFor(host string) (client dockerRegistryInterface, cancel context.CancelFunc, err error) {
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
	var transport http.RoundTripper
	{
		transport = &wwwAuthenticateFixer{transport: http.DefaultTransport}
		// Now the auth-handling wrappers that come with the library
		transport = dockerregistry.WrapTransport(transport, httphost, auth.username, auth.password)
		// Add the backoff mechanism so we don't DOS registries
		transport = BackoffRoundTripper(transport, initialBackoff, maxBackoff, clockwork.NewRealClock())
		// Add timeout context
		transport = &ContextRoundTripper{Transport: transport, Ctx: ctx}
		// Rate limit
		transport = RateLimitedRoundTripper(transport, f.rlConf, host)
	}

	client = herokuWrapper{
		&dockerregistry.Registry{
			URL: httphost,
			Client: &http.Client{
				Transport: transport,
				Jar:       jar,
				Timeout:   requestTimeout,
			},
			Logf: dockerregistry.Quiet,
		},
	}
	return
}

type ContextRoundTripper struct {
	Transport http.RoundTripper
	Ctx       context.Context
}

func (rt *ContextRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt.Transport.RoundTrip(r.WithContext(rt.Ctx))
}
