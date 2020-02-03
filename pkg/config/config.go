// config is the package containing configuration for fluxd, shared so
// it can be used by fluxd itself as well as other programs e.g.,
// `fluxctl install`.
package config

import (
	"fmt"
	"time"
)

const (
	ConfigPath        = "/etc/fluxd/conf"
	ConfigName        = "flux-config.yaml"
	ConfigType        = "yaml"
	FluxConfigVersion = "v1"
)

type Config struct {
	// This is expected to be present in a config file (and will not
	// correspond to a flag). The value determines how the config file
	// is interpreted: for now, if it is not equal to
	// FluxConfigVersion above, it is considered an invalid
	// configuration.
	ConfigVersion string `mapstructure:"fluxConfigVersion"`

	LogFormat     string `mapstructure:"logFormat"`
	Listen        string `mapstructure:"listen"`
	ListenMetrics string `mapstructure:"listenMetrics"`

	GitURL              string        `mapstructure:"gitUrl"`
	GitBranch           string        `mapstructure:"gitBranch"`
	GitPath             []string      `mapstructure:"gitPath"`
	GitReadonly         bool          `mapstructure:"gitReadonly"`
	GitUser             string        `mapstructure:"gitUser"`
	GitEmail            string        `mapstructure:"gitEmail"`
	GitSetAuthor        bool          `mapstructure:"gitSetAuthor"`
	GitLabel            string        `mapstructure:"gitLabel"`
	GitSecret           bool          `mapstructure:"gitSecret"`
	GitSyncTag          string        `mapstructure:"gitSyncTag"`
	GitNotesRef         string        `mapstructure:"gitNotesRef"`
	GitCISkip           bool          `mapstructure:"gitCiSkip"`
	GitCISkipMessage    string        `mapstructure:"gitCiSkipMessage"`
	GitPollInterval     time.Duration `mapstructure:"gitPollInterval"`
	GitTimeout          time.Duration `mapstructure:"gitTimeout"`
	GitGPGKeyImport     []string      `mapstructure:"gitGpgKeyImport"`
	GitVerifySignatures bool          `mapstructure:"gitVerifySignatures"`
	GitSigningKey       string        `mapstructure:"gitSigningKey"`

	SyncInterval             time.Duration `mapstructure:"syncInterval"`
	SyncTimeout              time.Duration `mapstructure:"syncTimeout"`
	SyncGarbageCollection    bool          `mapstructure:"syncGarbageCollection"`
	SyncGarbageCollectionDry bool          `mapstructure:"syncGarbageCollectionDry"`
	SyncState                string        `mapstructure:"syncState"`
	SopsEnabled              bool          `mapstructure:"sops"`

	RegistryDisableScanning bool `mapstructure:"registryDisableScanning"`

	MemcachedHostname string        `mapstructure:"memcachedHostname"`
	MemcachedPort     int           `mapstructure:"memcachedPort"`
	MemcachedService  string        `mapstructure:"memcachedService"`
	MemcachedTimeout  time.Duration `mapstructure:"memcachedTimeout"`

	AutomationInterval   time.Duration `mapstructure:"automationInterval"`
	RegistryPollInterval time.Duration `mapstructure:"registryPollInterval"`
	RegistryRPS          float64       `mapstructure:"registryRps"`
	RegistryBurst        int           `mapstructure:"registryBurst"`
	RegistryTrace        bool          `mapstructure:"registryTrace"`
	RegistryInsecureHost []string      `mapstructure:"registryInsecureHost"`
	RegistryExcludeImage []string      `mapstructure:"registryExcludeImage"`
	RegistryUseLabels    []string      `mapstructure:"registryUseLabels"`
	RegistryECRRegion    []string      `mapstructure:"registryEcrRegion"`
	RegistryECRIncludeID []string      `mapstructure:"registryEcrIncludeId"`
	RegistryECRExcludeID []string      `mapstructure:"registryEcrExcludeId"`
	RegistryRequire      []string      `mapstructure:"registryRequire"`

	K8sSecretName            string        `mapstructure:"k8sSecretName"`
	K8sSecretVolumeMountPath string        `mapstructure:"k8sSecretVolumeMountPath"`
	K8sSecretDataKey         string        `mapstructure:"k8sSecretDataKey"`
	K8sAllowNamespace        []string      `mapstructure:"k8sAllowNamespace"`
	K8sDefaultNamespace      string        `mapstructure:"k8sDefaultNamespace"`
	K8sExcludeResource       []string      `mapstructure:"k8sUnsafeExcludeResource"`
	K8sVerbosity             int           `mapstructure:"k8sVerbosity"`
	SSHKeygenDir             string        `mapstructure:"sshKeygenDir"`
	ManifestGeneration       bool          `mapstructure:"manifestGeneration"`
	Connect                  string        `mapstructure:"connect"`
	Token                    string        `mapstructure:"token"`
	RPCTimeout               time.Duration `mapstructure:"rpcTimeout"`
	DockerConfig             string        `mapstructure:"dockerConfig"`
}

func (c Config) IsValid() error {
	if c.ConfigVersion != FluxConfigVersion {
		return fmt.Errorf("config file is expected to include `fluxConfigVersion: %s` to mark it as a Flux config", FluxConfigVersion)
	}
	return nil
}
