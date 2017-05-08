package registry

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
)

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
