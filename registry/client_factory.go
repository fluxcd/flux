package registry

import (
	"context"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"time"

	"github.com/go-kit/kit/log"
	dockerregistry "github.com/heroku/docker-registry-client/registry"
	"golang.org/x/net/publicsuffix"

	"github.com/weaveworks/flux/registry/cache"
	"github.com/weaveworks/flux/registry/middleware"
)

var (
	ErrNoMemcache = errors.New("no memcached")
)

// ClientFactory creates a new client for the given host.
// Each request might require a new client. E.g. when retrieving docker
// images from docker hub, then a second from quay.io
type ClientFactory interface {
	ClientFor(host string, creds Credentials) (client Client, err error)
}

type RemoteClientFactory struct {
	Logger   log.Logger
	Limiters *middleware.RateLimiters
}

func (f *RemoteClientFactory) ClientFor(host string, creds Credentials) (Client, error) {
	httphost := "https://" + host

	// quay.io wants us to use cookies for authorisation, so we have
	// to construct one (the default client has none). This means a
	// bit more constructing things to be able to make a registry
	// client literal, rather than calling .New()
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, err
	}
	auth := creds.credsFor(host)

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)

	// Use the wrapper to fix headers for quay.io, and remember bearer tokens
	var transport http.RoundTripper
	{
		transport = &middleware.WWWAuthenticateFixer{Transport: http.DefaultTransport}
		// Now the auth-handling wrappers that come with the library
		transport = dockerregistry.WrapTransport(transport, httphost, auth.username, auth.password)
		// Add timeout context
		transport = &middleware.ContextRoundTripper{Transport: transport, Ctx: ctx}
		// Rate limit
		transport = f.Limiters.RoundTripper(transport, host)
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
func NewCacheClientFactory(l log.Logger, cache cache.Reader, cacheExpiry time.Duration) ClientFactory {
	return &cacheClientFactory{
		Logger:      l,
		cache:       cache,
		CacheExpiry: cacheExpiry,
	}
}

type cacheClientFactory struct {
	Logger      log.Logger
	cache       cache.Reader
	CacheExpiry time.Duration
}

func (f *cacheClientFactory) ClientFor(host string, creds Credentials) (Client, error) {
	if f.cache == nil {
		return nil, ErrNoMemcache
	}
	client := NewCache(creds, f.cache, f.CacheExpiry, f.Logger)
	return client, nil
}
