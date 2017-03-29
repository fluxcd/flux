package flux

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"strings"

	"golang.org/x/crypto/ssh"
	"reflect"
	"regexp"
)

const secretReplacement = "******"

var (
	settingCleaner  = regexp.MustCompile(`[']`)
	settingSplitter = regexp.MustCompile(`('.+'|[^.]+)`)
)

// Instance configuration, mutated via `fluxctl config`. It can be
// supplied as YAML (hence YAML annotations) and is transported as
// JSON (hence JSON annotations).

type GitConfig struct {
	URL    string `json:"URL" yaml:"URL"`
	Path   string `json:"path" yaml:"path"`
	Branch string `json:"branch" yaml:"branch"`
	Key    string `json:"key" yaml:"key"`
}

// NotifierConfig is the config used to set up a notifier.
type NotifierConfig struct {
	HookURL         string `json:"hookURL" yaml:"hookURL"`
	Username        string `json:"username" yaml:"username"`
	ReleaseTemplate string `json:"releaseTemplate" yaml:"releaseTemplate"`
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
	Slack    NotifierConfig `json:"slack" yaml:"slack"`
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

// FindSetting returns a reflect.Value representing the found setting.
// If the Value is invalid, the setting wasn't found.
// Only allow from SafeInstanceConfig to attempt to prevent leaking
// secure variables.
func (c SafeInstanceConfig) FindSetting(path, syntax string) reflect.Value {
	if syntax == "" {
		syntax = "yaml" // Default to yaml, to match `fluxctl get-config`
	}
	// Split path into configuration keys
	confKeys := settingSplitter.FindAllString(path, -1)

	// Get value for the configuration path
	w := configWalker{
		syntax,
	}
	return w.walk(reflect.ValueOf(c), confKeys...)
}

type configWalker struct {
	Syntax string
}

// walk recursively walks through a struct to find an element
// with the same json tag.
// E.g. git.url will first look for a top level
// field with a json tag called git, reflect into that struct and then
// find and return a field that has the json tag url.
func (w configWalker) walk(topValue reflect.Value, paths ...string) reflect.Value {
	// Clean path of any invalid character
	paths[0] = settingCleaner.ReplaceAllString(paths[0], "")

	var nextValue reflect.Value
	switch topValue.Kind() {
	case reflect.Struct:
		nextValue = w.valueFromStruct(topValue, paths[0])
		break
	case reflect.Map:
		nextValue = w.valueFromMap(topValue, paths[0])
		break
	}
	// If we continue to have valid values and there are more paths to search
	if nextValue.IsValid() && len(paths[1:]) > 0 {
		return w.walk(nextValue, paths[1:]...)
	}
	// Final value
	return nextValue
}

// Search fields for json tags
func (w configWalker) valueFromStruct(v reflect.Value, match string) reflect.Value {
	for i := 0; i < v.NumField(); i++ {
		vt := v.Type().Field(i)
		jsonTag := vt.Tag.Get(w.Syntax)
		if jsonTag == match {
			return v.Field(i)
		}
	}
	return reflect.Value{}
}

// Search for key in map
func (w configWalker) valueFromMap(v reflect.Value, match string) reflect.Value {
	m := v.Interface()
	switch m.(type) {
	case map[string]Auth:
		castMap := v.Interface().(map[string]Auth)
		for k, v := range castMap {
			if k == match {
				return reflect.ValueOf(v.Auth)
			}
		}
	}
	return reflect.Value{}
}
