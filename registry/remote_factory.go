// We can't inject a remote client directly because each repository request might be to a different
// registry provider. E.g. both docker hub and quay containers. So a new remote client must be
// created for each new Image. This factory provides that and can be mocked out.
package registry

import (
	"context"
	"github.com/go-kit/kit/log"
	"net/http"
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

func NewRemoteClientFactory(c Credentials, l log.Logger, mc MemcacheClient, ce time.Duration, rlc RateLimiterConfig) RemoteClientFactory {
	for host, creds := range c.m {
		l.Log("host", host, "username", creds.username)
	}
	return &remoteClientFactory{
		creds:          c,
		Logger:         l,
		MemcacheClient: mc,
		CacheExpiry:    ce,
		rlConf:         rlc,
	}
}

type remoteClientFactory struct {
	creds          Credentials
	Logger         log.Logger
	MemcacheClient MemcacheClient
	CacheExpiry    time.Duration
	rlConf         RateLimiterConfig
}

func (f *remoteClientFactory) CreateFor(host string) (_ Remote, err error) {
	client, cancel, err := f.newRegistryClient(host)
	if err != nil {
		return
	}
	return newRemote(client, cancel), nil
}

func (f *remoteClientFactory) newRegistryClient(host string) (client dockerRegistryInterface, cancel context.CancelFunc, err error) {
	if f.MemcacheClient != nil {
		client = NewCache(f.creds, f.MemcacheClient, f.CacheExpiry, f.Logger)
	} else {
		f.Logger.Log("registry_cache", "disabled")
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
