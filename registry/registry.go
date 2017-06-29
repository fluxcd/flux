// Package registry provides domain abstractions over container registries.
package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	officialMemcache "github.com/bradfitz/gomemcache/memcache"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/go-kit/kit/log"
	dockerregistry "github.com/heroku/docker-registry-client/registry"
	"github.com/jonboulle/clockwork"
	wraperrors "github.com/pkg/errors"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/registry/memcache"
	"github.com/weaveworks/flux/registry/middleware"
	"golang.org/x/net/publicsuffix"
	"net/http"
	"net/http/cookiejar"
	"sort"
	"strings"
	"time"
)

const (
	requestTimeout = 10 * time.Second
	maxConcurrency = 10 // Chosen arbitrarily
)

// The Registry interface is a domain specific API to access container registries.
type Registry interface {
	GetRepository(repository Repository) ([]flux.Image, error)
	GetImage(repository Repository, tag string) (flux.Image, error)
}

type registry struct {
	factory ClientFactory
	Logger  log.Logger
}

// NewClient creates a new registry registry, to use when fetching repositories.
func NewRegistry(c ClientFactory, l log.Logger) Registry {
	return &registry{
		factory: c,
		Logger:  l,
	}
}

// GetRepository yields a repository matching the given name, if any exists.
// Repository may be of various forms, in which case omitted elements take
// assumed defaults.
//
//   helloworld             -> index.docker.io/library/helloworld
//   foo/helloworld         -> index.docker.io/foo/helloworld
//   quay.io/foo/helloworld -> quay.io/foo/helloworld
//
func (reg *registry) GetRepository(img Repository) (_ []flux.Image, err error) {
	rem, err := reg.newRemote(img)
	if err != nil {
		return
	}

	tags, err := rem.Tags(img)
	if err != nil {
		rem.Cancel()
		return nil, err
	}

	// the hostlessImageName is canonicalised, in the sense that it
	// includes "library" as the org, if unqualified -- e.g.,
	// `library/nats`. We need that to fetch the tags etc. However, we
	// want the results to use the *actual* name of the images to be
	// as supplied, e.g., `nats`.
	return reg.tagsToRepository(rem, img, tags)
}

// Get a single Image from the registry if it exists
func (reg *registry) GetImage(img Repository, tag string) (_ flux.Image, err error) {
	rem, err := reg.newRemote(img)
	if err != nil {
		return
	}
	return rem.Manifest(img, tag)
}

func (reg *registry) newRemote(img Repository) (rem Remote, err error) {
	client, cancel, err := reg.factory.ClientFor(img.Host())
	if err != nil {
		return
	}
	rem = newRemote(client, cancel)
	rem = NewInstrumentedRemote(rem)
	return
}

func (reg *registry) tagsToRepository(remote Remote, repository Repository, tags []string) ([]flux.Image, error) {
	// one way or another, we'll be finishing all requests
	defer remote.Cancel()

	type result struct {
		image flux.Image
		err   error
	}

	toFetch := make(chan string, len(tags))
	fetched := make(chan result, len(tags))

	for i := 0; i < maxConcurrency; i++ {
		go func() {
			for tag := range toFetch {
				image, err := remote.Manifest(repository, tag)
				if err != nil {
					reg.Logger.Log("registry-metadata-err", err)
				}
				fetched <- result{image, err}
			}
		}()
	}
	for _, tag := range tags {
		toFetch <- tag
	}
	close(toFetch)

	images := make([]flux.Image, cap(fetched))
	for i := 0; i < cap(fetched); i++ {
		res := <-fetched
		if res.err != nil {
			return nil, res.err
		}
		images[i] = res.image
	}

	sort.Sort(byCreatedDesc(images))
	return images, nil
}

// -----

type byCreatedDesc []flux.Image

func (is byCreatedDesc) Len() int      { return len(is) }
func (is byCreatedDesc) Swap(i, j int) { is[i], is[j] = is[j], is[i] }
func (is byCreatedDesc) Less(i, j int) bool {
	switch {
	case is[i].CreatedAt.IsZero():
		return true
	case is[j].CreatedAt.IsZero():
		return false
	case is[i].CreatedAt.Equal(is[j].CreatedAt):
		return is[i].ID.String() < is[j].ID.String()
	default:
		return is[i].CreatedAt.After(is[j].CreatedAt)
	}
}

type Remote interface {
	Tags(repository Repository) ([]string, error)
	Manifest(repository Repository, tag string) (flux.Image, error)
	Cancel()
}

type remote struct {
	client dockerRegistryInterface
	cancel context.CancelFunc
}

func newRemote(client dockerRegistryInterface, cancel context.CancelFunc) Remote {
	return &remote{
		client: client,
		cancel: cancel,
	}
}

func (rc *remote) Tags(repository Repository) (_ []string, err error) {
	return rc.client.Tags(repository.String())
}

func (rc *remote) Manifest(repository Repository, tag string) (img flux.Image, err error) {
	img, err = flux.ParseImage(fmt.Sprintf("%s:%s", repository.String(), tag), time.Time{})
	if err != nil {
		return
	}
	history, err := rc.client.Manifest(repository.String(), tag)
	if err != nil {
		return
	}

	// the manifest includes some v1-backwards-compatibility data,
	// oddly called "History", which are layer metadata as JSON
	// strings; these appear most-recent (i.e., topmost layer) first,
	// so happily we can just decode the first entry to get a created
	// time.
	type v1image struct {
		Created time.Time `json:"created"`
	}
	var topmost v1image
	if len(history) > 0 {
		if err = json.Unmarshal([]byte(history[0].V1Compatibility), &topmost); err == nil {
			if !topmost.Created.IsZero() {
				img.CreatedAt = topmost.Created
			}
		}
	}

	return
}

func (rc *remote) Cancel() {
	rc.cancel()
}

// This is an interface that represents the heroku docker registry library
type dockerRegistryInterface interface {
	Tags(repository string) ([]string, error)
	Manifest(repository, reference string) ([]schema1.History, error)
}

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

func NewCacheClientFactory(c Credentials, l log.Logger, mc memcache.MemcacheClient, ce time.Duration) ClientFactory {
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
	MemcacheClient memcache.MemcacheClient
	CacheExpiry    time.Duration
}

func (f *cacheClientFactory) ClientFor(host string) (dockerRegistryInterface, context.CancelFunc, error) {
	if f.MemcacheClient == nil {
		return nil, nil, ErrNoMemcache
	}
	client := NewCache(f.creds, f.MemcacheClient, f.CacheExpiry, f.Logger)
	return client, func() {}, nil
}

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

func (f *remoteClientFactory) ClientFor(host string) (dockerRegistryInterface, context.CancelFunc, error) {
	httphost := "https://" + host

	// quay.io wants us to use cookies for authorisation, so we have
	// to construct one (the default client has none). This means a
	// bit more constructing things to be able to make a registry
	// client literal, rather than calling .New()
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, nil, err
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

	client := herokuWrapper{
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
	return client, cancel, nil
}

type Repository struct {
	img flux.Image // Internally we use an image to store data
}

func RepositoryFromImage(img flux.Image) Repository {
	return Repository{
		img: img,
	}
}

func ParseRepository(imgStr string) (Repository, error) {
	i, err := flux.ParseImage(imgStr, time.Time{})
	if err != nil {
		return Repository{}, err
	}
	return Repository{
		img: i,
	}, nil
}

func (r Repository) NamespaceImage() string {
	return r.img.ID.NamespaceImage()
}

func (r Repository) Host() string {
	return r.img.ID.Host
}

func (r Repository) String() string {
	return r.img.ID.HostNamespaceImage()
}

func (r Repository) ToImage(tag string) flux.Image {
	newImage := r.img
	newImage.ID.Tag = tag
	return newImage
}

type herokuWrapper struct {
	*dockerregistry.Registry
}

// Convert between types. dockerregistry returns the *same* type but from a
// vendored library so go freaks out. Eugh.
// TODO: The only thing we care about here for now is history. Frankly it might
// be easier to convert it to JSON and back.
func (h herokuWrapper) Manifest(repository, reference string) ([]schema1.History, error) {
	manifest, err := h.Registry.Manifest(repository, reference)
	if err != nil || manifest == nil {
		return nil, err
	}
	var result []schema1.History
	for _, item := range manifest.History {
		result = append(result, schema1.History{item.V1Compatibility})
	}
	return result, err
}

type Cache struct {
	creds  Credentials
	expiry time.Duration
	Client memcache.MemcacheClient
	logger log.Logger
}

func NewCache(creds Credentials, cache memcache.MemcacheClient, expiry time.Duration, logger log.Logger) dockerRegistryInterface {
	return &Cache{
		creds:  creds,
		expiry: expiry,
		Client: cache,
		logger: logger,
	}
}

func (c *Cache) Manifest(repository, reference string) (history []schema1.History, err error) {
	repo, err := ParseRepository(repository)
	if err != nil {
		c.logger.Log("err", wraperrors.Wrap(err, "Parsing repository"))
		return
	}
	creds := c.creds.credsFor(repo.Host())

	// Try the cache
	key := manifestKey(creds.username, repo.String(), reference)
	cacheItem, err := c.Client.Get(key)
	if err != nil {
		if err != officialMemcache.ErrCacheMiss {
			c.logger.Log("err", wraperrors.Wrap(err, "Fetching tag from memcache"))
		}
		return
	}

	// Return the cache item
	err = json.Unmarshal(cacheItem.Value, &history)
	if err != nil {
		c.logger.Log("err", err.Error)
		return
	}
	return
}

func (c *Cache) Tags(repository string) (tags []string, err error) {
	repo, err := ParseRepository(repository)
	if err != nil {
		c.logger.Log("err", wraperrors.Wrap(err, "Parsing repository"))
		return
	}
	creds := c.creds.credsFor(repo.Host())

	// Try the cache
	key := tagKey(creds.username, repo.String())
	cacheItem, err := c.Client.Get(key)
	if err != nil {
		if err != officialMemcache.ErrCacheMiss {
			c.logger.Log("err", wraperrors.Wrap(err, "Fetching tag from memcache"))
		}
		return
	}

	// Return the cache item
	err = json.Unmarshal(cacheItem.Value, &tags)
	if err != nil {
		c.logger.Log("err", err.Error)
		return
	}
	return
}

func manifestKey(username, repository, reference string) string {
	return strings.Join([]string{
		"registryhistoryv1", // Just to version in case we need to change format later.
		// Just the username here means we won't invalidate the cache when user
		// changes password, but that should be rare. And, it also means we're not
		// putting user passwords in plaintext into memcache.
		username,
		repository,
		reference,
	}, "|")
}

func tagKey(username, repository string) string {
	return strings.Join([]string{
		"registrytagsv1", // Just to version in case we need to change format later.
		// Just the username here means we won't invalidate the cache when user
		// changes password, but that should be rare. And, it also means we're not
		// putting user passwords in plaintext into memcache.
		username,
		repository,
	}, "|")
}
