package flux

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
	Auths map[string]struct {
		Auth string `json:"auth" yaml:"auth"`
	} `json:"auths" yaml:"auths"`
}

type InstanceConfig struct {
	Git      GitConfig      `json:"git" yaml:"git"`
	Slack    SlackConfig    `json:"slack" yaml:"slack"`
	Registry RegistryConfig `json:"registry" yaml:"registry"`
}

func (c InstanceConfig) HideSecrets() InstanceConfig {
	if c.Git.Key != "" {
		c.Git.Key = secretReplacement
	}
	for host, auth := range c.Registry.Auths {
		auth.Auth = secretReplacement
		c.Registry.Auths[host] = auth
	}
	return c
}
