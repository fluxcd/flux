package registry

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/url"
	"strings"
)

// Registry Credentials
type creds struct {
	username, password string
}

// Credentials to a (Docker) registry.
type Credentials struct {
	m map[string]creds
}

// NoCredentials returns a usable but empty credentials object.
func NoCredentials() Credentials {
	return Credentials{
		m: map[string]creds{},
	}
}

func CredentialsFromFile(path string) (Credentials, error) {
	configBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return Credentials{}, err
	}

	var config struct {
		Auths map[string]struct {
			Auth string
		}
	}
	if err = json.Unmarshal(configBytes, &config); err != nil {
		return Credentials{}, err
	}

	m := map[string]creds{}
	for host, entry := range config.Auths {
		decodedAuth, err := base64.StdEncoding.DecodeString(entry.Auth)
		if err != nil {
			return Credentials{}, err
		}
		authParts := strings.SplitN(string(decodedAuth), ":", 2)
		if len(authParts) != 2 {
			return Credentials{},
				fmt.Errorf("decoded credential for %v has wrong number of fields (expected 2, got %d)", host, len(authParts))
		}

		// Some users were passing in credentials in the form of
		// http://docker.io and http://docker.io/v1/, etc.
		// So strip everything down to it's base host.
		u, err := url.Parse(host)
		if err != nil {
			return Credentials{}, err
		}
		if u.Host == "" && u.Path == "" {
			return Credentials{}, errors.New("Empty registry auth url")
		}
		if u.Host == "" { // If there's no https:// prefix, it won't parse the host.
			u, err = url.Parse(fmt.Sprintf("https://%s/", host))
			if err != nil {
				return Credentials{}, err
			}
			// If the host is still empty, then there's probably a rogue /
			if u.Host == "" {
				return Credentials{}, errors.New("Invalid registry auth url. Must be a valid http address (e.g. https://gcr.io/v1/)")
			}
		}
		host = u.Host

		m[host] = creds{
			username: authParts[0],
			password: authParts[1],
		}
	}
	return Credentials{m: m}, nil
}

// For yields an authenticator for a specific host.
func (cs Credentials) credsFor(host string) creds {
	if cred, found := cs.m[host]; found {
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
