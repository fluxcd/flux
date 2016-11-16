package flux

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"strings"

	"golang.org/x/crypto/ssh"
)

const secretReplacement = "******"

// Instance configuration, mutated via `fluxctl config`. It can be
// supplied as YAML (hence YAML annotations) and is transported as
// JSON (hence JSON annotations).

type GitConfig struct {
	URL    string `json:"URL" yaml:"URL"`
	Path   string `json:"path" yaml:"path"`
	Branch string `json:"branch" yaml:"branch"`
	Key    string `json:"key" yaml:"key"`
}

type SlackConfig struct {
	HookURL  string `json:"hookURL" yaml:"hookURL"`
	Username string `json:"username" yaml:"username"`
}

type RegistryConfig struct {
	// Map of index host to Basic auth string (base64 encoded
	// username:password), to make it easy to copypasta from docker
	// config.
	Auths map[string]Auth `json:"auths" yaml:"auths"`
}

type Auth struct {
	Auth string `json:"auth" yaml:"auth"`
}

type InstanceConfig struct {
	Git      GitConfig      `json:"git" yaml:"git"`
	Slack    SlackConfig    `json:"slack" yaml:"slack"`
	Registry RegistryConfig `json:"registry" yaml:"registry"`
}

// As a safeguard, we make the default behaviour to hide secrets when
// marshalling config.

type SafeInstanceConfig InstanceConfig
type UnsafeInstanceConfig InstanceConfig

func (c InstanceConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.HideSecrets())
}

func (c InstanceConfig) HideSecrets() SafeInstanceConfig {
	c.Git = c.Git.HideKey()
	for host, auth := range c.Registry.Auths {
		c.Registry.Auths[host] = auth.HidePassword()
	}
	return SafeInstanceConfig(c)
}

func (a Auth) HidePassword() Auth {
	if a.Auth == "" {
		return a
	}
	bytes, err := base64.StdEncoding.DecodeString(a.Auth)
	if err != nil {
		return Auth{secretReplacement}
	}
	parts := strings.SplitN(string(bytes), ":", 2)
	return Auth{parts[0] + ":" + secretReplacement}
}

func (g GitConfig) HideKey() GitConfig {
	if g.Key == "" {
		return g
	}
	key, err := ssh.ParseRawPrivateKey([]byte(g.Key))
	if err != nil {
		g.Key = secretReplacement
		return g
	}

	privKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		g.Key = secretReplacement
		return g
	}

	pubKey, err := ssh.NewPublicKey(&privKey.PublicKey)
	if err != nil {
		g.Key = secretReplacement
		return g
	}

	g.Key = string(ssh.MarshalAuthorizedKey(pubKey))
	return g
}
