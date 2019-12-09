// config is the package containing configuration for fluxd, shared so
// it can be used by fluxd itself as well as other programs e.g.,
// `fluxctl install`.
package config

import (
	"time"
)

const ConfigPath = "/etc/fluxd/conf"
const ConfigName = "flux-config"

type Config struct {
	LogFormat     string `mapstructure:"log-format"`
	Listen        string `mapstructure:"listen"`
	ListenMetrics string `mapstructure:"listen-metrics"`

	GitURL              string        `mapstructure:"git-url"`
	GitBranch           string        `mapstructure:"git-branch"`
	GitPath             []string      `mapstructure:"git-path"`
	GitReadonly         bool          `mapstructure:"git-readonly"`
	GitUser             string        `mapstructure:"git-user"`
	GitEmail            string        `mapstructure:"git-email"`
	GitSetAuthor        bool          `mapstructure:"git-set-author"`
	GitLabel            string        `mapstructure:"git-label"`
	GitSecret           bool          `mapstructure:"git-secret"`
	GitSyncTag          string        `mapstructure:"git-sync-tag"`
	GitNotesRef         string        `mapstructure:"git-notes-ref"`
	GitCISkip           bool          `mapstructure:"git-ci-skip"`
	GitCISkipMessage    string        `mapstructure:"git-ci-skip-message"`
	GitPollInterval     time.Duration `mapstructure:"git-poll-interval"`
	GitTimeout          time.Duration `mapstructure:"git-timeout"`
	GitGPGKeyImport     []string      `mapstructure:"git-gpg-key-import"`
	GitVerifySignatures bool          `mapstructure:"git-verify-signatures"`
	GitSigningKey       string        `mapstructure:"git-signing-key"`

	SyncInterval             time.Duration `mapstructure:"sync-interval"`
	SyncTimeout              time.Duration `mapstructure:"sync-timeout"`
	SyncGarbageCollection    bool          `mapstructure:"sync-garbage-collection"`
	SyncGarbageCollectionDry bool          `mapstructure:"sync-garbage-collection-dry"`
	SyncState                string        `mapstructure:"sync-state"`
	SopsEnabled              bool          `mapstructure:"sops"`

	RegistryDisableScanning bool `mapstructure:"registry-disable-scanning"`

	MemcachedHostname string        `mapstructure:"memcached-hostname"`
	MemcachedPort     int           `mapstructure:"memcached-port"`
	MemcachedService  string        `mapstructure:"memcached-service"`
	MemcachedTimeout  time.Duration `mapstructure:"memcached-timeout"`

	AutomationInterval   time.Duration `mapstructure:"automation-interval"`
	RegistryPollInterval time.Duration `mapstructure:"registry-poll-interval"`
	RegistryRPS          float64       `mapstructure:"registry-rps"`
	RegistryBurst        int           `mapstructure:"registry-burst"`
	RegistryTrace        bool          `mapstructure:"registry-trace"`
	RegistryInsecureHost []string      `mapstructure:"registry-insecure-host"`
	RegistryExcludeImage []string      `mapstructure:"registry-exclude-image"`
	RegistryUseLabels    []string      `mapstructure:"registry-use-labels"`
	RegistryECRRegion    []string      `mapstructure:"registry-ecr-region"`
	RegistryECRIncludeID []string      `mapstructure:"registry-ecr-include-id"`
	RegistryECRExcludeID []string      `mapstructure:"registry-ecr-exclude-id"`
	RegistryRequire      []string      `mapstructure:"registry-require"`

	K8sSecretName            string        `mapstructure:"k8s-secret-name"`
	K8sSecretVolumeMountPath string        `mapstructure:"k8s-secret-volume-mount-path"`
	K8sSecretDataKey         string        `mapstructure:"k8s-secret-data-key"`
	K8sAllowNamespace        []string      `mapstructure:"k8s-allow-namespace"`
	K8sDefaultNamespace      string        `mapstructure:"k8s-default-namespace"`
	K8sExcludeResource       []string      `mapstructure:"k8s-unsafe-exclude-resource"`
	K8sVerbosity             int           `mapstructure:"k8s-verbosity"`
	SSHKeygenDir             string        `mapstructure:"ssh-keygen-dir"`
	ManifestGeneration       bool          `mapstructure:"manifest-generation"`
	Connect                  string        `mapstructure:"connect"`
	Token                    string        `mapstructure:"token"`
	RPCTimeout               time.Duration `mapstructure:"rpc-timeout"`
	DockerConfig             string        `mapstructure:"docker-config"`
}
