package registry

import (
	"net/http"
	"net/http/cookiejar"

	"github.com/go-kit/kit/log"
	dockerregistry "github.com/heroku/docker-registry-client/registry"
	"golang.org/x/net/publicsuffix"

	"github.com/weaveworks/flux/registry/middleware"
)

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

	// Use the wrapper to fix headers for quay.io, and remember bearer tokens
	var transport http.RoundTripper
	{
		transport = &middleware.WWWAuthenticateFixer{Transport: http.DefaultTransport}
		// Now the auth-handling wrappers that come with the library
		transport = dockerregistry.WrapTransport(transport, httphost, auth.username, auth.password)
		// Rate limit
		transport = f.Limiters.RoundTripper(transport, host)
	}

	registry := &dockerregistry.Registry{
		URL: httphost,
		Client: &http.Client{
			Transport: transport,
			Jar:       jar,
			Timeout:   requestTimeout,
		},
		Logf: dockerregistry.Quiet,
	}
	client := &Remote{
		Registry: registry,
	}
	return NewInstrumentedClient(client), nil
}
