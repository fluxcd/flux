// Package registry provides domain abstractions over container registries.
package registry

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strings"
	"time"

	dockerregistry "github.com/CenturyLinkLabs/docker-reg-client/registry"
	"github.com/go-kit/kit/log"
	"golang.org/x/net/publicsuffix"
)

const (
	dockerHubHost    = "index.docker.io"
	dockerHubLibrary = "library"
)

// Credentials to a (Docker) registry.
type Credentials map[string]dockerregistry.Authenticator

// Client is a handle to a registry.
type Client struct {
	Credentials Credentials
	Logger      log.Logger
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

	// quay.io wants us to use cookies for authorisation; the registry
	// client uses http.DefaultClient, so happily we can splat a
	// cookie jar into the default client and it'll work.
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, err
	}
	http.DefaultClient.Jar = jar
	client := dockerregistry.NewClient()

	if host != dockerHubHost {
		baseURL, err := url.Parse(fmt.Sprintf("https://%s/v1/", host))
		if err != nil {
			return nil, err
		}
		client.BaseURL = baseURL
	}

	auth0 := c.Credentials.For(host)
	// NB index.docker.io needs this because it's an "index registry";
	// quay.io needs this because this is where it sets the session
	// cookie it wants for authorisation later.
	auth, err := client.Hub.GetReadTokenWithAuth(hostlessImageName, auth0)
	if err != nil {
		return nil, err
	}

	tags, err := client.Repository.ListTags(hostlessImageName, auth)
	if err != nil {
		return nil, err
	}

	return c.tagsToRepository(client, repository, tags, auth), nil
}

func (c *Client) lookupImage(client *dockerregistry.Client, repoName, tag, ID string, auth dockerregistry.Authenticator) Image {
	img := ParseImage(repoName)
	img.Tag = tag
	meta, err := client.Image.GetMetadata(ID, auth)
	if err != nil {
		c.Logger.Log("registry-metadata-err", err)
	} else {
		img.CreatedAt = meta.Created
	}
	return img
}

func (c *Client) tagsToRepository(client *dockerregistry.Client, repoName string, tags map[string]string, auth dockerregistry.Authenticator) *Repository {
	fetched := make(chan Image, len(tags))

	for tag, imageID := range tags {
		go func(t, id string) {
			fetched <- c.lookupImage(client, repoName, t, id, auth)
		}(tag, imageID)
	}

	images := make([]Image, cap(fetched))
	for i := 0; i < cap(fetched); i++ {
		images[i] = <-fetched
	}

	sort.Sort(byCreatedDesc{images})

	return &Repository{
		Name:   repoName,
		Images: images,
	}
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

// NoCredentials returns a usable but empty credentials object.
func NoCredentials() Credentials {
	return make(map[string]dockerregistry.Authenticator)
}

// CredentialsFromFile returns a credentials object parsed from the given
// filepath.
func CredentialsFromFile(path string) (Credentials, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config dockerConfig
	if err = json.Unmarshal(bytes, &config); err != nil {
		return nil, err
	}

	creds := make(map[string]dockerregistry.Authenticator)
	for host, entry := range config.Auths {
		decodedAuth, err := base64.StdEncoding.DecodeString(entry.Auth)
		if err != nil {
			return nil, err
		}
		authParts := strings.SplitN(string(decodedAuth), ":", 2)
		creds[host] = &dockerregistry.BasicAuth{
			Username: authParts[0],
			Password: authParts[1],
		}
	}
	return creds, nil
}

// For yields an authenticator for a specific host.
func (cs Credentials) For(host string) dockerregistry.Authenticator {
	if auth, found := cs[host]; found {
		return auth
	}
	if auth, found := cs[fmt.Sprintf("https://%s/v1/", host)]; found {
		return auth
	}
	return dockerregistry.NilAuth{}
}

// Hosts returns all of the hosts available in these credentials.
func (cs Credentials) Hosts() []string {
	hosts := []string{}
	for host := range cs {
		hosts = append(hosts, host)
	}
	return hosts
}

// -----

type auth struct {
	Auth  string `json:"auth"`
	Email string `json:"email"`
}

type dockerConfig struct {
	Auths map[string]auth `json:"auths"`
}

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
