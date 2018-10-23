package registry

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"

	"github.com/pkg/errors"
	"github.com/weaveworks/flux/sidecar/aws/ecr"
)

// credential to a Docker registry.
type credential struct {
	username, password   string
	registry, provenance string
}

func (c credential) String() string {
	if (credential{}) == c {
		return "<zero creds>"
	}
	return fmt.Sprintf("<registry creds for %s@%s, from %s>", c.username, c.registry, c.provenance)
}

// Credentials to a (Docker) registry.
type Credentials struct {
	m map[string]credential
}

// NoCredentials returns a usable but empty credentials object.
func NoCredentials() Credentials {
	return Credentials{
		m: map[string]credential{},
	}
}

func ParseCredentials(from string, b []byte) (Credentials, error) {
	var config struct {
		Auths map[string]struct {
			Auth string
		}
	}
	if err := json.Unmarshal(b, &config); err != nil {
		return Credentials{}, err
	}
	// If it's in k8s format, it won't have the surrounding "Auth". Try that too.
	if len(config.Auths) == 0 {
		if err := json.Unmarshal(b, &config.Auths); err != nil {
			return Credentials{}, err
		}
	}
	m := map[string]credential{}
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
		// So strip everything down to the host.
		// Also, the registry might be local and on a different port.
		// So we need to check for that because url.Parse won't parse the ip:port format very well.
		u, err := url.Parse(host)
		if err != nil {
			return Credentials{}, err
		}
		if u.Host == "" && u.Path == "" && !strings.Contains(host, ":") || host == "http://" || host == "https://" {
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

		m[host] = credential{
			registry:   host,
			provenance: from,
			username:   authParts[0],
			password:   authParts[1],
		}
	}
	return Credentials{m: m}, nil
}

// ImageCredsWithDefaults designates the appropriate credential to use for a repository
// using a docker configuration file supplied by the caller.
func ImageCredsWithDefaults(lookup func() ImageCreds, configPath string) (func() ImageCreds, error) {
	// pre-flight check
	bs, err := ioutil.ReadFile(configPath)
	if err == nil {
		_, err = ParseCredentials(configPath, bs)
	}
	if err != nil {
		return nil, err
	}

	return func() ImageCreds {
		var defaults Credentials
		bs, err := ioutil.ReadFile(configPath)
		if err == nil {
			defaults, _ = ParseCredentials(configPath, bs)
		}
		imageCreds := lookup()
		for k, v := range imageCreds {
			newCreds := NoCredentials()
			newCreds.Merge(defaults)
			newCreds.Merge(v)
			imageCreds[k] = newCreds
		}
		return imageCreds
	}, nil
}

// ---

// credsFor yields an authenticator for a specific host.
func (cs Credentials) credsFor(host string) credential {
	if cred, found := cs.m[host]; found {
		return cred
	}
	if host == "gcr.io" || strings.HasSuffix(host, ".gcr.io") {
		if cred, err := GetGCPOauthToken(host); err == nil {
			return cred
		}
	}
	// WIP: Not tested to work successfully yet..
	// The idea is to fetch the authentication token from the ecr sidecar's http url
	if strings.HasSuffix(host, "amazonaws.com") {
		if cred, err := GetECRRegistryCredential(host, ecr.SidecarAWSURL); err == nil {
			return cred
		}
	}
	return credential{}
}

// Hosts returns all of the hosts available in these credentials.
func (cs Credentials) Hosts() []string {
	hosts := []string{}
	for host := range cs.m {
		hosts = append(hosts, host)
	}
	return hosts
}

func (cs Credentials) Merge(c Credentials) {
	for k, v := range c.m {
		cs.m[k] = v
	}
}

func (cs Credentials) String() string {
	return fmt.Sprintf("{%v}", cs.m)
}
