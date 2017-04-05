package flux

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"strings"

	"errors"
	"golang.org/x/crypto/ssh"
	"reflect"
	"regexp"
)

const secretReplacement = "******"

var (
	settingCleaner   = regexp.MustCompile(`[']`)
	settingSplitter  = regexp.MustCompile(`('.+'|[^.]+)`)
	settingDelimiter = "."
	mapKeyWrapper    = byte('\'')
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

type ConfigSyntax string

type SingleConfigParams struct {
	Key    string
	Syntax ConfigSyntax
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

// WriteSetting will set a value in an InstanceConfig struct for a given key.
// The reference to the config must be a pointer so it can update values in place.
func (c *InstanceConfig) WriteSetting(setting SingleConfigParams, value string) error {
	if setting.Syntax == "" {
		setting.Syntax = "yaml" // Default to yaml, to match `fluxctl get-config`
	}

	// Get value for the configuration path
	w := configWalker{
		string(setting.Syntax),
	}

	path := setting.Key
	v := reflect.ValueOf(c)
	splitPaths := settingSplitter.FindAllString(path, -1)
	// Only walk up to the first map. Then have custom code to write to specific maps
	// Really hacky way of doing it, but I've no better idea right now.
	lastIndex := 0
	for ; lastIndex < len(splitPaths); lastIndex++ {
		v = w.walk(v, splitPaths[lastIndex])
		if v.Kind() == reflect.Map {
			break
		}
	}

	if !v.CanSet() {
		return Missing{
			BaseError: &BaseError{
				Help: "The requested configuration parameter does not exist. Please ensure your request matches the configuration from `fluxctl get-config`",
				Err:  errors.New("Configuration parameter does not exist"),
			},
		}
	}

	// Special set for maps
	switch v.Kind() {
	case reflect.Map:
		m := v.Interface()
		match := settingCleaner.ReplaceAllString(splitPaths[lastIndex+1], "")
		switch m.(type) {
		case map[string]Auth:
			castMap := v.Interface().(map[string]Auth)
			newMap := make(map[string]Auth, len(castMap))
			for k, v := range castMap {
				if k == match {
					newMap[k] = Auth{value}
				} else {
					newMap[k] = v
				}
			}
			v.Set(reflect.ValueOf(newMap))
		default:
			panic("Setting map of type " + v.Kind().String() + " is not implemented")
		}
		break
	default:
		v.SetString(value)
	}
	return nil
}

// FindSetting returns a reflect.Value representing the found setting.
// If the Value is invalid, the setting wasn't found.
// Only allow from SafeInstanceConfig to attempt to prevent leaking
// secure variables.
func (c SafeInstanceConfig) FindSetting(path, syntax string) reflect.Value {
	if syntax == "" {
		syntax = "yaml" // Default to yaml, to key `fluxctl get-config`
	}
	// Split path into configuration keys
	confKeys := settingSplitter.FindAllString(path, -1)

	// Get value for the configuration path
	w := configWalker{
		syntax,
	}
	v := w.walk(reflect.ValueOf(c), confKeys...)

	// Special get for maps
	if v.Kind() == reflect.Map {
		m := v.Interface()
		paths := settingSplitter.FindAllString(path, -1)
		key := settingCleaner.ReplaceAllString(paths[len(paths)-1], "")
		switch m.(type) {
		case map[string]Auth:
			castMap := v.Interface().(map[string]Auth)
			v = reflect.ValueOf(castMap[key].Auth)
		default:
			panic("Getting map of type " + v.Kind().String() + " is not implemented")
		}
	}
	return v
}

type configWalker struct {
	Syntax string
}

// walk recursively walks through a struct to find an element
// with the same json tag.
// E.g. git.url will first look for a top level
// field with a json tag called git, reflect into that struct and then
// find and return a field that has the json tag url.
func (w configWalker) walk(v reflect.Value, paths ...string) reflect.Value {
	nextPath := paths[0]
	paths = paths[1:]
	nextValue := reflect.Value{}
	switch unwrap(v).Kind() {
	case reflect.Struct:
		for i := 0; i < unwrap(v).NumField(); i++ {
			vt := unwrap(v).Type().Field(i)
			tag := vt.Tag.Get(w.Syntax)
			if tag == nextPath {
				nextValue = unwrap(v).Field(i)
				break
			}
		}
		break
	case reflect.Map:
		m := v.Interface()
		key := settingCleaner.ReplaceAllString(nextPath, "")
		switch m.(type) {
		case map[string]Auth:
			castMap := v.Interface().(map[string]Auth)
			nextValue = reflect.ValueOf(castMap[key])
		default:
			panic("Getting map of type " + v.Kind().String() + " is not implemented")
		}
		break
	}
	if len(paths) > 0 {
		return w.walk(nextValue, paths...)
	}
	return nextValue
}

// unwrap is a helper to dereference values if they need to be.
// To write values, values must be a pointer to a variable. But to read
// from them they need to be dereferenced.
func unwrap(v reflect.Value) reflect.Value {
	if v.IsValid() && !v.CanAddr() && v.Kind() != reflect.Struct && v.Kind() != reflect.Map {
		return v.Elem()
	}
	return v
}

// Search fields for json tags
func (w configWalker) valueFromStruct(v reflect.Value, match string) reflect.Value {
	for i := 0; i < unwrap(v).NumField(); i++ {
		vt := unwrap(v).Type().Field(i)
		tag := vt.Tag.Get(w.Syntax)
		if tag == match {
			return unwrap(v).Field(i)
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
