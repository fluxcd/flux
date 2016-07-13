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
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"golang.org/x/net/publicsuffix"

	dockerregistry "github.com/CenturyLinkLabs/docker-reg-client/registry"
)

const DockerHubHost = "index.docker.io"
const DockerHubLibrary = "library"

type Credentials map[string]dockerregistry.Authenticator

type Client struct {
	Credentials Credentials
	Logger      log.Logger

	*dockerregistry.Client
}

type Error struct {
	*dockerregistry.RegistryError
}

func mkError(registryError error) error {
	return Error{registryError.(*dockerregistry.RegistryError)}
}

func (c *Client) GetRepository(imageName string) (*Repository, error) {
	var host, org, image string
	parts := strings.Split(imageName, "/")
	switch len(parts) {
	case 1:
		host = DockerHubHost
		org = DockerHubLibrary
		image = parts[0]
	case 2:
		host = DockerHubHost
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
	jar, err := cookiejar.New(&cookiejar.Options{publicsuffix.List})
	if err != nil {
		return nil, err
	}
	http.DefaultClient.Jar = jar
	c.Client = dockerregistry.NewClient()

	if host != DockerHubHost {
		baseURL, err := url.Parse(fmt.Sprintf("https://%s/v1/", host))
		if err != nil {
			return nil, err
		}
		c.BaseURL = baseURL
	}

	auth0 := c.Credentials.For(host)
	// NB index.docker.io needs this because it's an "index registry";
	// quay.io needs this because this is where it sets the session
	// cookie it wants for authorisation later.
	auth, err := c.Hub.GetReadTokenWithAuth(hostlessImageName, auth0)
	if err != nil {
		return nil, mkError(err)
	}

	tags, err := c.Repository.ListTags(hostlessImageName, auth)
	if err != nil {
		return nil, mkError(err)
	}

	return c.tagsToRepository(imageName, tags, auth), nil
}

// Repository is a collection of images with the same name
// (e.g,. "weaveworks/helloworld") but not the same tag (e.g.,
// "weaveworks/helloworld:v0.1").
type Repository struct {
	Name   string // "weaveworks/helloworld"
	Images []Image
}

// Image represents a specific container image available in a repository. It's a
// struct because I think we can safely assume the data here is pretty
// universal across different registries and repositories.
type Image struct {
	Name      string    // "weaveworks/helloworld"
	Tag       string    // "master-59f0001"
	CreatedAt time.Time // Always UTC
}

type Images []Image

func (is Images) Len() int      { return len(is) }
func (is Images) Swap(i, j int) { is[i], is[j] = is[j], is[i] }

type ImagesByCreatedDesc struct {
	Images
}

func (is ImagesByCreatedDesc) Less(i, j int) bool {
	return is.Images[i].CreatedAt.After(is.Images[j].CreatedAt)
}

func NoCredentials() Credentials {
	return make(map[string]dockerregistry.Authenticator)
}

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

func (cs Credentials) For(host string) dockerregistry.Authenticator {
	if auth, found := cs[host]; found {
		return auth
	}
	if auth, found := cs[fmt.Sprintf("https://%s/v1/", host)]; found {
		return auth
	}
	return dockerregistry.NilAuth{}
}

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

func (c *Client) lookupImage(imageName, tag, ID string, auth dockerregistry.Authenticator) Image {
	var createdAt time.Time
	meta, err := c.Image.GetMetadata(ID, auth)
	if err != nil {
		c.Logger.Log("registry-metadata-err", err)
	} else {
		createdAt = meta.Created
	}
	return Image{
		Name:      imageName,
		Tag:       tag,
		CreatedAt: createdAt,
	}
}

func (c *Client) tagsToRepository(imageName string, tags map[string]string, auth dockerregistry.Authenticator) *Repository {
	fetched := make(chan Image, len(tags))

	for tag, imageID := range tags {
		go func(t, id string) {
			fetched <- c.lookupImage(imageName, t, id, auth)
		}(tag, imageID)
	}

	images := make([]Image, cap(fetched))
	for i := 0; i < cap(fetched); i++ {
		images[i] = <-fetched
	}

	return &Repository{
		Name:   imageName,
		Images: images,
	}
}
