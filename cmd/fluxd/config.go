package main

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/fluxcd/flux/pkg/config"
	"github.com/fluxcd/flux/pkg/registry"
	"github.com/fluxcd/flux/pkg/sync"
)

// defineConfigFlags defines the flags that can also be set in
// a config file. These need special treatment, because some care must
// be taken to match them ("bind") with config file field names.
func defineConfigFlags(fs *pflag.FlagSet, bail func(error)) {

	bind := func(fieldName, flagName string) error {
		configStruct := reflect.TypeOf(config.Config{})
		field, ok := configStruct.FieldByName(fieldName)
		if !ok {
			return fmt.Errorf("attempt to bind a flag to a field not present in config.Config, %q", fieldName)
		}
		tag := field.Tag
		// this parallels the logic in
		// github.com/mitchellh/mapstructure, except that we want to
		// bail if a field is mentioned that is marked ignore, like
		// this: `mapstructure:"-"`
		mappedName := field.Name
		mapstructureTagParts := strings.Split(tag.Get("mapstructure"), ",")
		if namePart := mapstructureTagParts[0]; namePart != "" {
			if namePart == "-" { // means ignore this field
				return fmt.Errorf(`attempt to bind a flag to a config field tagged as ignored, %q`, field.Name)
			}
			mappedName = namePart
		}
		return viper.BindPFlag(mappedName, fs.Lookup(flagName))
	}

	bindOrBail := func(flagName, fieldName string) {
		if err := bind(flagName, fieldName); err != nil {
			bail(err)
		}
	}

	defineString := func(fieldName, flagName, def, desc string) {
		fs.String(flagName, def, desc)
		bindOrBail(fieldName, flagName)
	}

	defineStringP := func(fieldName, flagName, short, def, desc string) {
		fs.StringP(flagName, short, def, desc)
		bindOrBail(fieldName, flagName)
	}

	defineStringSlice := func(fieldName, flagName string, def []string, desc string) {
		fs.StringSlice(flagName, def, desc)
		bindOrBail(fieldName, flagName)
	}

	defineBool := func(fieldName, flagName string, def bool, desc string) {
		fs.Bool(flagName, def, desc)
		bindOrBail(fieldName, flagName)
	}

	defineDuration := func(fieldName, flagName string, def time.Duration, desc string) {
		fs.Duration(flagName, def, desc)
		bindOrBail(fieldName, flagName)
	}

	defineInt := func(fieldName, flagName string, def int, desc string) {
		fs.Int(flagName, def, desc)
		bindOrBail(fieldName, flagName)
	}

	defineFloat64 := func(fieldName, flagName string, def float64, desc string) {
		fs.Float64(flagName, def, desc)
		bindOrBail(fieldName, flagName)
	}

	defineString("LogFormat", "log-format", "fmt", "change the log format.")
	defineStringP("Listen", "listen", "l", ":3030", "listen address where /metrics and API will be served")
	defineString("ListenMetrics", "listen-metrics", "", "listen address for /metrics endpoint")

	// Git repo & key etc.
	defineString("GitURL", "git-url", "", "URL of git repo with Kubernetes manifests; e.g., git@github.com:weaveworks/flux-get-started")
	defineString("GitBranch", "git-branch", "master", "branch of git repo to use for Kubernetes manifests")
	defineStringSlice("GitPath", "git-path", []string{}, "relative paths within the git repo to locate Kubernetes manifests")
	defineBool("GitReadonly", "git-readonly", false, fmt.Sprintf("use to prevent Flux from pushing changes to git; implies --sync-state=%s", sync.NativeStateMode))
	defineString("GitUser", "git-user", "Weave Flux", "username to use as git committer")
	defineString("GitEmail", "git-email", "support@weave.works", "email to use as git committer")
	defineBool("GitSetAuthor", "git-set-author", false, "if set, the author of git commits will reflect the user who initiated the commit and will differ from the git committer.")
	defineString("GitLabel", "git-label", "", "label to keep track of sync progress; overrides both --git-sync-tag and --git-notes-ref")
	defineBool("GitSecret", "git-secret", false, `if set, git-secret will be run on every git checkout. A gpg key must be imported using  --git-gpg-key-import or by mounting a keyring containing it directly`)

	// Old git config; still used if --git-label is not supplied, but --git-label is preferred.
	defineString("GitSyncTag", "git-sync-tag", defaultGitSyncTag, fmt.Sprintf("tag to use to mark sync progress for this cluster (only relevant when --sync-state=%s)", sync.GitTagStateMode))
	defineString("GitNotesRef", "git-notes-ref", defaultGitNotesRef, "ref to use for keeping commit annotations in git notes")

	defineBool("GitCISkip", "git-ci-skip", false, `append "[ci skip]" to commit messages so that CI will skip builds`)
	defineString("GitCISkipMessage", "git-ci-skip-message", "", "additional text for commit messages, useful for skipping builds in CI. Use this to supply specific text, or set --git-ci-skip")
	defineDuration("GitPollInterval", "git-poll-interval", 5*time.Minute, "period at which to poll git repo for new commits")
	defineDuration("GitTimeout", "git-timeout", 20*time.Second, "duration after which git operations time out")

	// GPG commit signing
	defineStringSlice("GitGPGKeyImport", "git-gpg-key-import", []string{}, "keys at the paths given will be imported for use of signing and verifying commits")
	defineString("GitSigningKey", "git-signing-key", "", "if set, commits Flux makes will be signed with this GPG key")
	defineBool("GitVerifySignatures", "git-verify-signatures", false, "if set, the signature of commits will be verified before Flux applies them")

	// syncing
	defineDuration("SyncInterval", "sync-interval", 5*time.Minute, "apply config in git to cluster at least this often, even if there are no new commits")
	defineDuration("SyncTimeout", "sync-timeout", 1*time.Minute, "duration after which sync operations time out")
	defineBool("SyncGarbageCollection", "sync-garbage-collection", false, "experimental; delete resources that were created by fluxd, but are no longer in the git repo")
	defineBool("SyncGarbageCollectionDry", "sync-garbage-collection-dry", false, "experimental; only log what would be garbage collected, rather than deleting. Implies --sync-garbage-collection")
	defineString("SyncState", "sync-state", sync.GitTagStateMode, fmt.Sprintf("method used by flux for storing state (one of {%s})", strings.Join([]string{sync.GitTagStateMode, sync.NativeStateMode}, ",")))
	defineBool("SopsEnabled", "sops", false, `if set, decrypt SOPS-encrypted manifest files with before syncing them. Provide decryption keys in the same way you would provide them for the sops binary. Be aware that manifests generated with .flux.yaml are not automatically decrypted`)

	// registry
	defineBool("RegistryDisableScanning", "registry-disable-scanning", false, "do not scan container image registries to fill in the registry cache")

	defineString("MemcachedHostname", "memcached-hostname", "memcached", "hostname for memcached service.")
	defineInt("MemcachedPort", "memcached-port", 11211, "memcached service port.")
	defineDuration("MemcachedTimeout", "memcached-timeout", time.Second, "maximum time to wait before giving up on memcached requests.")
	defineString("MemcachedService", "memcached-service", "memcached", "SRV service used to discover memcache servers.")

	defineDuration("AutomationInterval", "automation-interval", 5*time.Minute, "period at which to check for image updates for automated workloads")
	defineFloat64("RegistryRPS", "registry-rps", 50, "maximum registry requests per second per host")
	defineInt("RegistryBurst", "registry-burst", defaultRemoteConnections, "maximum number of warmer connections to remote and memcache")
	defineBool("RegistryTrace", "registry-trace", false, "output trace of image registry requests to log")
	defineStringSlice("RegistryInsecureHost", "registry-insecure-host", []string{}, "let these registry hosts skip TLS host verification and fall back to using HTTP instead of HTTPS; this allows man-in-the-middle attacks, so use with extreme caution")
	defineStringSlice("RegistryExcludeImage", "registry-exclude-image", []string{"k8s.gcr.io/*"}, "do not scan images that match these glob expressions; the default is to exclude the 'k8s.gcr.io/*' images")
	defineStringSlice("RegistryUseLabels", "registry-use-labels", []string{"index.docker.io/weaveworks/*", "index.docker.io/fluxcd/*"}, "use the timestamp (RFC3339) from labels for (canonical) image refs that match these glob expression")

	// AWS authentication
	defineStringSlice("RegistryECRRegion", "registry-ecr-region", nil, "include just these AWS regions when scanning images in ECR; when not supplied, the cluster's region will included if it can be detected through the AWS API")
	defineStringSlice("RegistryECRIncludeID", "registry-ecr-include-id", nil, "restrict ECR scanning to these AWS account IDs; if not supplied, all account IDs that aren't excluded may be scanned")
	defineStringSlice("RegistryECRExcludeID", "registry-ecr-exclude-id", []string{registry.EKS_SYSTEM_ACCOUNT}, "do not scan ECR for images in these AWS account IDs; the default is to exclude the EKS system account")

	defineStringSlice("RegistryRequire", "registry-require", nil, fmt.Sprintf(`exit with an error if auto-authentication with any of the given registries is not possible (possible values: {%s})`, strings.Join(RequireValues, ",")))

	// k8s-secret backed ssh keyring configuration
	defineString("K8sSecretName", "k8s-secret-name", "flux-git-deploy", "name of the k8s secret used to store the private SSH key")
	defineString("K8sSecretVolumeMountPath", "k8s-secret-volume-mount-path", "/etc/fluxd/ssh", "mount location of the k8s secret storing the private SSH key")
	defineString("K8sSecretDataKey", "k8s-secret-data-key", "identity", "data key holding the private SSH key within the k8s secret")
	defineStringSlice("K8sAllowNamespace", "k8s-allow-namespace", []string{}, "experimental: restrict all operations to the provided namespaces")
	defineString("K8sDefaultNamespace", "k8s-default-namespace", "", "the namespace to use for resources where a namespace is not specified")
	defineStringSlice("K8sExcludeResource", "k8s-unsafe-exclude-resource", []string{"*metrics.k8s.io/*", "webhook.certmanager.k8s.io/*", "v1/Event"}, "do not attempt to obtain cluster resources whose group/version/kind matches these glob expressions. Potentially unsafe, please read its documentation first")
	defineInt("K8sVerbosity", "k8s-verbosity", 0, "klog verbosity level")
	defineString("SSHKeygenDir", "ssh-keygen-dir", "", "directory, ideally on a tmpfs volume, in which to generate new SSH keys when necessary")

	defineBool("ManifestGeneration", "manifest-generation", false, "experimental; search for .flux.yaml files to generate manifests")

	// upstream connection settings
	defineString("Connect", "connect", "", "connect to an upstream service e.g., Weave Cloud, at this base address")
	defineString("Token", "token", "", "authentication token for upstream service")
	defineDuration("RPCTimeout", "rpc-timeout", 10*time.Second, "maximum time an operation requested by the upstream may take")

	defineString("DockerConfig", "docker-config", "", "path to a docker config to use for image registry credentials")
}
