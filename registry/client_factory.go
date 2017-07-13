package registry

import (
	"context"
	"errors"
	"github.com/go-kit/kit/log"
	dockerregistry "github.com/heroku/docker-registry-client/registry"
	"github.com/jonboulle/clockwork"
	"github.com/weaveworks/flux/registry/cache"
	"github.com/weaveworks/flux/registry/middleware"
	"golang.org/x/net/publicsuffix"
	"net/http"
	"net/http/cookiejar"
	"time"
)

var (
	ErrNoMemcache = errors.New("no memecache")
)

// ClientFactory creates a new client for the given host.
// Each request might require a new client. E.g. when retrieving docker
// images from docker hub, then a second from quay.io
type ClientFactory interface {
	ClientFor(host string) (client Client, err error)
}

// ---
// A new ClientFactory for a Remote.
func NewRemoteClientFactory(c Credentials, l log.Logger, rlc middleware.RateLimiterConfig) ClientFactory {
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
	rlConf middleware.RateLimiterConfig
}

func (f *remoteClientFactory) ClientFor(host string) (Client, error) {
	httphost := "https://" + host

	// quay.io wants us to use cookies for authorisation, so we have
	// to construct one (the default client has none). This means a
	// bit more constructing things to be able to make a registry
	// client literal, rather than calling .New()
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, err
	}
	auth := f.creds.credsFor(host)

	// A context we'll use to cancel requests on error
	ctx, cancel := context.WithCancel(context.Background())
	// Add a timeout to the request
	ctx, cancel = context.WithTimeout(ctx, requestTimeout)

	// Use the wrapper to fix headers for quay.io, and remember bearer tokens
	var transport http.RoundTripper
	{
		transport = &middleware.WWWAuthenticateFixer{Transport: http.DefaultTransport}
		// Now the auth-handling wrappers that come with the library
		transport = dockerregistry.WrapTransport(transport, httphost, auth.username, auth.password)
		// Add the backoff mechanism so we don't DOS registries
		transport = middleware.BackoffRoundTripper(transport, middleware.InitialBackoff, middleware.MaxBackoff, clockwork.NewRealClock())
		// Add timeout context
		transport = &middleware.ContextRoundTripper{Transport: transport, Ctx: ctx}
		// Rate limit
		transport = middleware.RateLimitedRoundTripper(transport, f.rlConf, host)
	}

	herokuRegistry := herokuManifestAdaptor{
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
	client := &Remote{
		Registry:   &herokuRegistry,
		CancelFunc: cancel,
	}
	return NewInstrumentedClient(client), nil
}

// ---
// A new ClientFactory implementation for a Cache
func NewCacheClientFactory(c Credentials, l log.Logger, cache cache.Reader, cacheExpiry time.Duration) ClientFactory {
	for host, creds := range c.m {
		l.Log("host", host, "username", creds.username)
	}
	return &cacheClientFactory{
		creds:       c,
		Logger:      l,
		cache:       cache,
		CacheExpiry: cacheExpiry,
	}
}

type cacheClientFactory struct {
	creds       Credentials
	Logger      log.Logger
	cache       cache.Reader
	CacheExpiry time.Duration
}

func (f *cacheClientFactory) ClientFor(host string) (Client, error) {
	if f.cache == nil {
		return nil, ErrNoMemcache
	}
	client := NewCache(f.creds, f.cache, f.CacheExpiry, f.Logger)
	return client, nil
}
