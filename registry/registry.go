// Package registry provides domain abstractions over container registries.
package registry

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"sort"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	dockerregistry "github.com/heroku/docker-registry-client/registry"
	"golang.org/x/net/publicsuffix"
)

const (
	dockerHubHost    = "index.docker.io"
	dockerHubLibrary = "library"
)

type creds struct {
	username, password string
}

// Credentials to a (Docker) registry.
type Credentials struct {
	m map[string]creds
}

// Client is a handle to a registry.
type Client struct {
	Credentials Credentials
	Logger      log.Logger
}

type roundtripperFunc func(*http.Request) (*http.Response, error)

func (f roundtripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// GetRepository yields a repository matching the given name, if any exists.
// Repository may be of various forms, in which case omitted elements take
// assumed defaults.
//
//   helloworld             -> index.docker.io/library/helloworld
//   foo/helloworld         -> index.docker.io/foo/helloworld
//   quay.io/foo/helloworld -> quay.io/foo/helloworld
//
func (c *Client) GetRepository(repository string) (*Repository, error) {
	var host, org, image string
	parts := strings.Split(repository, "/")
	switch len(parts) {
	case 1:
		host = dockerHubHost
		org = dockerHubLibrary
		image = parts[0]
	case 2:
		host = dockerHubHost
		org = parts[0]
		image = parts[1]
	case 3:
		host = parts[0]
		org = parts[1]
		image = parts[2]
	default:
		return nil, fmt.Errorf(`expected image name as either "<host>/<org>/<image>", "<org>/<image>", or "<image>"`)
	}

	hostlessImageName := fmt.Sprintf("%s/%s", org, image)
	httphost := "https://" + host

	// quay.io wants us to use cookies for authorisation, so we have
	// to construct one (the default client has none). This means a
	// bit more constructing things to be able to make a registry
	// client literal, rather than calling .New()
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, err
	}
	auth := c.Credentials.credsFor(host)

	// A context we'll use to cancel requests on error
	ctx, cancel := context.WithCancel(context.Background())

	// Use the wrapper to fix headers for quay.io, and remember bearer tokens
	var transport http.RoundTripper = &wwwAuthenticateFixer{transport: http.DefaultTransport}
	// Now the auth-handling wrappers that come with the library
	transport = dockerregistry.WrapTransport(transport, httphost, auth.username, auth.password)

	client := &dockerregistry.Registry{
		URL: httphost,
		Client: &http.Client{
			Transport: roundtripperFunc(func(r *http.Request) (*http.Response, error) {
				return transport.RoundTrip(r.WithContext(ctx))
			}),
			Jar: jar,
		},
		Logf: dockerregistry.Quiet,
	}

	tags, err := client.Tags(hostlessImageName)
	if err != nil {
		cancel()
		return nil, err
	}

	return c.tagsToRepository(cancel, client, repository, tags)
}

func (c *Client) lookupImage(client *dockerregistry.Registry, repoName, tag string) (Image, error) {
	img := ParseImage(repoName)
	img.Tag = tag
	meta, err := client.Manifest(img.Name, tag)
	if err != nil {
		return img, err
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
	if err = json.Unmarshal([]byte(meta.History[0].V1Compatibility), &topmost); err == nil {
		img.CreatedAt = topmost.Created
	}

	return img, err
}

func (c *Client) tagsToRepository(cancel func(), client *dockerregistry.Registry, repoName string, tags []string) (*Repository, error) {
	// one way or another, we'll be finishing all requests
	defer cancel()

	type result struct {
		image Image
		err   error
	}

	fetched := make(chan result, len(tags))

	for _, tag := range tags {
		go func(t string) {
			img, err := c.lookupImage(client, repoName, t)
			if err != nil {
				c.Logger.Log("registry-metadata-err", err)
			}
			fetched <- result{img, err}
		}(tag)
	}

	images := make([]Image, cap(fetched))
	for i := 0; i < cap(fetched); i++ {
		res := <-fetched
		if res.err != nil {
			return nil, res.err
		}
		images[i] = res.image
	}

	sort.Sort(byCreatedDesc{images})

	return &Repository{
		Name:   repoName,
		Images: images,
	}, nil
}

// Repository is a collection of images with the same registry and name
// (e.g,. "quay.io:5000/weaveworks/helloworld") but not the same tag (e.g.,
// "quay.io:5000/weaveworks/helloworld:v0.1").
type Repository struct {
	Name   string // "quay.io:5000/weaveworks/helloworld"
	Images []Image
}

// Image represents a specific container image available in a repository. It's a
// struct because I think we can safely assume the data here is pretty
// universal across different registries and repositories.
type Image struct {
	Registry  string    // "quay.io:5000"
	Name      string    // "weaveworks/helloworld"
	Tag       string    // "master-59f0001"
	CreatedAt time.Time // Always UTC
}

// ParseImage splits the image string apart, returning an Image with as much
// info as we can gather.
func ParseImage(image string) (i Image) {
	parts := strings.SplitN(image, "/", 3)
	if len(parts) == 3 {
		i.Registry = parts[0]
		image = fmt.Sprintf("%s/%s", parts[1], parts[2])
	}
	parts = strings.SplitN(image, ":", 2)
	if len(parts) == 2 {
		i.Tag = parts[1]
	}
	i.Name = parts[0]
	return i
}

// String prints as much of an image as we have in the typical docker format. e.g. registry/name:tag
func (i Image) String() string {
	s := i.Repository()
	if i.Tag != "" {
		s = s + ":" + i.Tag
	}
	return s
}

// Repository returns a string with as much info as we have to rebuild the
// image repository (i.e. registry/name)
func (i Image) Repository() string {
	repo := i.Name
	if i.Registry != "" {
		repo = i.Registry + "/" + repo
	}
	return repo
}

// --- Credentials

// NoCredentials returns a usable but empty credentials object.
func NoCredentials() Credentials {
	return Credentials{
		m: map[string]creds{},
	}
}

// CredentialsFromFile returns a credentials object parsed from the given
// filepath.
func CredentialsFromFile(path string) (Credentials, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return Credentials{}, err
	}

	type dockerConfig struct {
		Auths map[string]struct {
			Auth  string `json:"auth"`
			Email string `json:"email"`
		} `json:"auths"`
	}

	var config dockerConfig
	if err = json.Unmarshal(bytes, &config); err != nil {
		return Credentials{}, err
	}

	credentials := Credentials{
		m: map[string]creds{},
	}
	for host, entry := range config.Auths {
		decodedAuth, err := base64.StdEncoding.DecodeString(entry.Auth)
		if err != nil {
			return Credentials{}, err
		}
		authParts := strings.SplitN(string(decodedAuth), ":", 2)
		credentials.m[host] = creds{
			username: authParts[0],
			password: authParts[1],
		}
	}
	return credentials, nil
}

// For yields an authenticator for a specific host.
func (cs Credentials) credsFor(host string) creds {
	if cred, found := cs.m[host]; found {
		return cred
	}
	if cred, found := cs.m[fmt.Sprintf("https://%s/v1/", host)]; found {
		return cred
	}
	return creds{}
}

// Hosts returns all of the hosts available in these credentials.
func (cs Credentials) Hosts() []string {
	hosts := []string{}
	for host := range cs.m {
		hosts = append(hosts, host)
	}
	return hosts
}

// -----

type images []Image

func (is images) Len() int      { return len(is) }
func (is images) Swap(i, j int) { is[i], is[j] = is[j], is[i] }

type byCreatedDesc struct{ images }

func (is byCreatedDesc) Less(i, j int) bool {
	if is.images[i].CreatedAt.Equal(is.images[j].CreatedAt) {
		return is.images[i].String() < is.images[j].String()
	}
	return is.images[i].CreatedAt.After(is.images[j].CreatedAt)
}

// Log requests as they go through, and responses as they come back.
// transport = logTransport{
// 	transport: transport,
// 	log: func(format string, args ...interface{}) {
// 		c.Logger.Log("registry-client-log", fmt.Sprintf(format, args...))
// 	},
// }
type logTransport struct {
	log       func(string, ...interface{})
	transport http.RoundTripper
}

func (t logTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.log("Request %s %#v", req.URL, req)
	res, err := t.transport.RoundTrip(req)
	t.log("Response %#v", res)
	if err != nil {
		t.log("Error %s", err)
	}
	return res, err
}

// ==== TO REMOVE

// LatestImage returns the latest releasable image from the repository.
// A releasable image is one that is not tagged "latest".
// Images must be kept in newest-first order.
func (r Repository) LatestImage() (Image, error) {
	for _, image := range r.Images {
		if strings.EqualFold(image.Tag, "latest") {
			continue
		}
		return image, nil
	}
	return Image{}, errors.New("no valid images in repository")
}
