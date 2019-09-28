package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	helmopclient "github.com/fluxcd/helm-operator/pkg/client/clientset/versioned"
	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	crd "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8sruntime "k8s.io/apimachinery/pkg/util/runtime"
	k8sclientdynamic "k8s.io/client-go/dynamic"
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	hrclient "github.com/fluxcd/flux/integrations/client/clientset/versioned"
	"github.com/fluxcd/flux/pkg/checkpoint"
	"github.com/fluxcd/flux/pkg/cluster"
	"github.com/fluxcd/flux/pkg/cluster/kubernetes"
	"github.com/fluxcd/flux/pkg/daemon"
	"github.com/fluxcd/flux/pkg/git"
	"github.com/fluxcd/flux/pkg/gpg"
	transport "github.com/fluxcd/flux/pkg/http"
	"github.com/fluxcd/flux/pkg/http/client"
	daemonhttp "github.com/fluxcd/flux/pkg/http/daemon"
	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/job"
	"github.com/fluxcd/flux/pkg/manifests"
	"github.com/fluxcd/flux/pkg/registry"
	"github.com/fluxcd/flux/pkg/registry/cache"
	registryMemcache "github.com/fluxcd/flux/pkg/registry/cache/memcached"
	registryMiddleware "github.com/fluxcd/flux/pkg/registry/middleware"
	"github.com/fluxcd/flux/pkg/remote"
	"github.com/fluxcd/flux/pkg/ssh"
	fluxsync "github.com/fluxcd/flux/pkg/sync"
)

var version = "unversioned"

const (
	product = "weave-flux"

	// This is used as the "burst" value for rate limiting, and
	// therefore also as the limit to the number of concurrent fetches
	// and memcached connections, since these in general can't do any
	// more work than is allowed by the burst amount.
	defaultRemoteConnections = 10

	// There are running systems that assume these defaults (by not
	// supplying a value for one or both). Don't change them.
	defaultGitSyncTag     = "flux-sync"
	defaultGitNotesRef    = "flux"
	defaultGitSkipMessage = "\n\n[ci skip]"

	RequireECR = "ecr"
)

var (
	RequireValues = []string{RequireECR}
)

type Config struct {
	LogFormat                string        `mapstructure:"log-format"`
	Listen                   string        `mapstructure:"listen"`
	ListenMetrics            string        `mapstructure:"listen-metrics"`
	KubernetesKubectl        string        `mapstructure:"kubernetes-kubectl"`
	GitURL                   string        `mapstructure:"git-url"`
	GitBranch                string        `mapstructure:"git-branch"`
	GitPath                  []string      `mapstructure:"git-path"`
	GitReadonly              bool          `mapstructure:"git-readonly"`
	GitUser                  string        `mapstructure:"git-user"`
	GitEmail                 string        `mapstructure:"git-email"`
	GitSetAuthor             bool          `mapstructure:"git-set-author"`
	GitLabel                 string        `mapstructure:"git-label"`
	GitSecret                bool          `mapstructure:"git-secret"`
	GitSyncTag               string        `mapstructure:"git-sync-tag"`
	GitNotesRef              string        `mapstructure:"git-notes-ref"`
	GitCISkip                bool          `mapstructure:"git-ci-skip"`
	GitCISkipMessage         string        `mapstructure:"git-ci-skip-message"`
	GitPollInterval          time.Duration `mapstructure:"git-poll-interval"`
	GitTimeout               time.Duration `mapstructure:"git-timeout"`
	GitGPGKeyImport          []string      `mapstructure:"git-gpg-key-import"`
	GitVerifySignatures      bool          `mapstructure:"git-verify-signatures"`
	GitSigningKey            string        `mapstructure:"git-signing-key"`
	SyncInterval             time.Duration `mapstructure:"sync-interval"`
	SyncTimeout              time.Duration `mapstructure:"sync-timeout"`
	SyncGarbageCollection    bool          `mapstructure:"sync-garbage-collection"`
	SyncGarbageCollectionDry bool          `mapstructure:"sync-garbage-collection-dry"`
	SyncState                string        `mapstructure:"sync-state"`
	MemcachedHostname        string        `mapstructure:"memcached-hostname"`
	MemcachedPort            int           `mapstructure:"memcached-port"`
	MemcachedService         string        `mapstructure:"memcached-service"`
	MemcachedTimeout         time.Duration `mapstructure:"memcached-timeout"`
	AutomationInterval       time.Duration `mapstructure:"automation-interval"`
	RegistryPollInterval     time.Duration `mapstructure:"registry-poll-interval"`
	RegistryRPS              float64       `mapstructure:"registry-rps"`
	RegistryBurst            int           `mapstructure:"registry-burst"`
	RegistryTrace            bool          `mapstructure:"registry-trace"`
	RegistryInsecureHost     []string      `mapstructure:"registry-insecure-host"`
	RegistryExcludeImage     []string      `mapstructure:"registry-exclude-image"`
	RegistryUseLabels        []string      `mapstructure:"registry-use-labels"`
	RegistryECRRegion        []string      `mapstructure:"registry-ecr-region"`
	RegistryECRIncludeID     []string      `mapstructure:"registry-ecr-include-id"`
	RegistryECRExcludeID     []string      `mapstructure:"registry-ecr-exclude-id"`
	RegistryRequire          []string      `mapstructure:"registry-require"`
	K8sInCluster             bool          `mapstructure:"k8s-in-cluster"`
	K8sSecretName            string        `mapstructure:"k8s-secret-name"`
	K8sSecretVolumeMountPath string        `mapstructure:"k8s-secret-volume-mount-path"`
	K8sSecretDataKey         string        `mapstructure:"k8s-secret-data-key"`
	K8sNamespaceWhitelist    []string      `mapstructure:"k8s-namespace-whitelist"`
	K8sAllowNamespace        []string      `mapstructure:"k8s-allow-namespace"`
	K8sVerbosity             int           `mapstructure:"k8s-verbosity"`
	SSHKeygenDir             string        `mapstructure:"ssh-keygen-dir"`
	ManifestGeneration       bool          `mapstructure:"manifest-generation"`
	Connect                  string        `mapstructure:"connect"`
	Token                    string        `mapstructure:"token"`
	RPCTimeout               time.Duration `mapstructure:"rpc-timeout"`
	Version                  bool          `mapstructure:"version"`
	RegistryCacheExpiry      string        `mapstructure:"registry-cache-expiry"`
	DockerConfig             string        `mapstructure:"docker-config"`
}

func optionalVar(fs *pflag.FlagSet, value ssh.OptionalValue, name, usage string) ssh.OptionalValue {
	fs.Var(value, name, usage)
	return value
}

type stringset []string

func (set stringset) has(possible string) bool {
	for _, s := range set {
		if s == possible {
			return true
		}
	}
	return false
}

func main() {
	// Flag domain.
	fs := pflag.NewFlagSet("default", pflag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "DESCRIPTION\n")
		fmt.Fprintf(os.Stderr, "  fluxd is the agent of flux.\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "FLAGS\n")
		fs.PrintDefaults()
	}

	_ = fs.String("log-format", "fmt", "change the log format.")
	_ = fs.StringP("listen", "l", ":3030", "listen address where /metrics and API will be served")

	_ = fs.String("listen-metrics", "", "listen address for /metrics endpoint")
	_ = fs.String("kubernetes-kubectl", "", "optional, explicit path to kubectl tool")
	// Git repo & key etc.
	_ = fs.String("git-url", "", "URL of git repo with Kubernetes manifests; e.g., git@github.com:weaveworks/flux-get-started")
	_ = fs.String("git-branch", "master", "branch of git repo to use for Kubernetes manifests")
	_ = fs.StringSlice("git-path", []string{}, "relative paths within the git repo to locate Kubernetes manifests")
	_ = fs.Bool("git-readonly", false, fmt.Sprintf("use to prevent Flux from pushing changes to git; implies --sync-state=%s", fluxsync.NativeStateMode))
	_ = fs.String("git-user", "Weave Flux", "username to use as git committer")
	_ = fs.String("git-email", "support@weave.works", "email to use as git committer")
	_ = fs.Bool("git-set-author", false, "if set, the author of git commits will reflect the user who initiated the commit and will differ from the git committer.")
	_ = fs.String("git-label", "", "label to keep track of sync progress; overrides both --git-sync-tag and --git-notes-ref")
	_ = fs.Bool("git-secret", false, `if set, git-secret will be run on every git checkout. A gpg key must be imported using  --git-gpg-key-import or by mounting a keyring containing it directly`)
	// Old git config; still used if --git-label is not supplied, but --git-label is preferred.
	_ = fs.String("git-sync-tag", defaultGitSyncTag, fmt.Sprintf("tag to use to mark sync progress for this cluster (only relevant when --sync-state=%s)", fluxsync.GitTagStateMode))
	_ = fs.String("git-notes-ref", defaultGitNotesRef, "ref to use for keeping commit annotations in git notes")
	_ = fs.Bool("git-ci-skip", false, `append "[ci skip]" to commit messages so that CI will skip builds`)
	_ = fs.String("git-ci-skip-message", "", "additional text for commit messages, useful for skipping builds in CI. Use this to supply specific text, or set --git-ci-skip")

	_ = fs.Duration("git-poll-interval", 5*time.Minute, "period at which to poll git repo for new commits")
	_ = fs.Duration("git-timeout", 20*time.Second, "duration after which git operations time out")

	// GPG commit signing
	_ = fs.StringSlice("git-gpg-key-import", []string{}, "keys at the paths given will be imported for use of signing and verifying commits")
	_ = fs.String("git-signing-key", "", "if set, commits Flux makes will be signed with this GPG key")
	_ = fs.Bool("git-verify-signatures", false, "if set, the signature of commits will be verified before Flux applies them")

	// syncing
	_ = fs.Duration("sync-interval", 5*time.Minute, "apply config in git to cluster at least this often, even if there are no new commits")
	_ = fs.Duration("sync-timeout", 1*time.Minute, "duration after which sync operations time out")
	_ = fs.Bool("sync-garbage-collection", false, "experimental; delete resources that were created by fluxd, but are no longer in the git repo")
	_ = fs.Bool("sync-garbage-collection-dry", false, "experimental; only log what would be garbage collected, rather than deleting. Implies --sync-garbage-collection")
	_ = fs.String("sync-state", fluxsync.GitTagStateMode, fmt.Sprintf("method used by flux for storing state (one of {%s})", strings.Join([]string{fluxsync.GitTagStateMode, fluxsync.NativeStateMode}, ",")))

	// registry
	_ = fs.String("memcached-hostname", "memcached", "hostname for memcached service.")
	_ = fs.Int("memcached-port", 11211, "memcached service port.")
	_ = fs.Duration("memcached-timeout", time.Second, "maximum time to wait before giving up on memcached requests.")
	_ = fs.String("memcached-service", "memcached", "SRV service used to discover memcache servers.")

	_ = fs.Duration("automation-interval", 5*time.Minute, "period at which to check for image updates for automated workloads")
	_ = fs.Duration("registry-poll-interval", 5*time.Minute, "period at which to check for updated images")
	_ = fs.Float64("registry-rps", 50, "maximum registry requests per second per host")
	_ = fs.Int("registry-burst", defaultRemoteConnections, "maximum number of warmer connections to remote and memcache")
	_ = fs.Bool("registry-trace", false, "output trace of image registry requests to log")
	_ = fs.StringSlice("registry-insecure-host", []string{}, "let these registry hosts skip TLS host verification and fall back to using HTTP instead of HTTPS; this allows man-in-the-middle attacks, so use with extreme caution")
	_ = fs.StringSlice("registry-exclude-image", []string{"k8s.gcr.io/*"}, "do not scan images that match these glob expressions; the default is to exclude the 'k8s.gcr.io/*' images")
	_ = fs.StringSlice("registry-use-labels", []string{"index.docker.io/weaveworks/*", "index.docker.io/fluxcd/*"}, "use the timestamp (RFC3339) from labels for (canonical) image refs that match these glob expression")

	// AWS authentication
	_ = fs.StringSlice("registry-ecr-region", nil, "include just these AWS regions when scanning images in ECR; when not supplied, the cluster's region will included if it can be detected through the AWS API")
	_ = fs.StringSlice("registry-ecr-include-id", nil, "restrict ECR scanning to these AWS account IDs; if not supplied, all account IDs that aren't excluded may be scanned")
	_ = fs.StringSlice("registry-ecr-exclude-id", []string{registry.EKS_SYSTEM_ACCOUNT}, "do not scan ECR for images in these AWS account IDs; the default is to exclude the EKS system account")

	_ = fs.StringSlice("registry-require", nil, fmt.Sprintf(`exit with an error if auto-authentication with any of the given registries is not possible (possible values: {%s})`, strings.Join(RequireValues, ",")))

	// k8s-secret backed ssh keyring configuration
	_ = fs.Bool("k8s-in-cluster", true, "set this to true if fluxd is deployed as a container inside Kubernetes")
	_ = fs.String("k8s-secret-name", "flux-git-deploy", "name of the k8s secret used to store the private SSH key")
	_ = fs.String("k8s-secret-volume-mount-path", "/etc/fluxd/ssh", "mount location of the k8s secret storing the private SSH key")
	_ = fs.String("k8s-secret-data-key", "identity", "data key holding the private SSH key within the k8s secret")
	_ = fs.StringSlice("k8s-namespace-whitelist", []string{}, "experimental, optional: restrict the view of the cluster to the namespaces listed. All namespaces are included if this is not set")
	_ = fs.StringSlice("k8s-allow-namespace", []string{}, "experimental: restrict all operations to the provided namespaces")
	_ = fs.Int("k8s-verbosity", 0, "klog verbosity level")
	_ = fs.String("ssh-keygen-dir", "", "directory, ideally on a tmpfs volume, in which to generate new SSH keys when necessary")

	_ = fs.Bool("manifest-generation", false, "experimental; search for .flux.yaml files to generate manifests")

	// upstream connection settings
	_ = fs.String("connect", "", "connect to an upstream service e.g., Weave Cloud, at this base address")
	_ = fs.String("token", "", "authentication token for upstream service")
	_ = fs.Duration("rpc-timeout", 10*time.Second, "maximum time an operation requested by the upstream may take")

	_ = fs.String("docker-config", "", "path to a docker config to use for image registry credentials")

	// This mirrors how kubectl extracts information from the environment.
	var (
		versionFlag = fs.Bool("version", false, "get version number")

		// SSH key generation
		sshKeyBits = optionalVar(fs, &ssh.KeyBitsValue{}, "ssh-keygen-bits", "-b argument to ssh-keygen (default unspecified)")
		sshKeyType = optionalVar(fs, &ssh.KeyTypeValue{}, "ssh-keygen-type", "-t argument to ssh-keygen (default unspecified)")

		_ = fs.Duration("registry-cache-expiry", 0, "")
	)
	fs.MarkDeprecated("registry-cache-expiry", "no longer used; cache entries are expired adaptively according to how often they change")
	fs.MarkDeprecated("k8s-namespace-whitelist", "changed to --k8s-allow-namespace, use that instead")
	fs.MarkDeprecated("registry-poll-interval", "changed to --automation-interval, use that instead")

	var kubeConfig *string
	{
		// Set the default kube config
		if home := homeDir(); home != "" {
			kubeConfig = fs.String("kube-config", filepath.Join(home, ".kube", "config"), "the absolute path of the k8s config file.")
		} else {
			kubeConfig = fs.String("kube-config", "", "the absolute path of the k8s config file.")
		}
	}

	// Explicitly initialize klog to enable stderr logging,
	// and parse our own flags.
	klogFlags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(klogFlags)

	err := fs.Parse(os.Args[1:])

	// Configure viper to check for a environment variables or a config file
	// environment variables will override config file variables
	viper.SetEnvPrefix("FLUXCD")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
	viper.SetConfigName("flux-config")
	viper.AddConfigPath("/etc/fluxd/")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Printf("config file not found: %s\n", err.Error())
		} else {
			fmt.Printf("error loading config file: %s\n", err.Error())
		}
	} else {
		fmt.Printf("using configuration from /etc/fluxd/flux-config.yaml with command-line overrides\n")
	}
	// Bind Viper to the pflags defined above
	viper.BindPFlags(fs)
	var config Config
	viper.Unmarshal(&config)

	switch {
	case err == pflag.ErrHelp:
		os.Exit(0)
	case err != nil:
		fmt.Fprintf(os.Stderr, "Error: %s\n\n", err.Error())
		fs.Usage()
		os.Exit(2)
	case *versionFlag:
		fmt.Println(version)
		os.Exit(0)
	}

	// set klog verbosity level
	if config.K8sVerbosity > 0 {
		verbosity := klogFlags.Lookup("v")
		verbosity.Value.Set(strconv.Itoa(config.K8sVerbosity))
		klog.V(4).Infof("Kubernetes client verbosity level set to %v", klogFlags.Lookup("v").Value)
	}

	// Logger component.
	var logger log.Logger
	{
		switch config.LogFormat {
		case "json":
			logger = log.NewJSONLogger(log.NewSyncWriter(os.Stderr))
		case "fmt":
			logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
		default:
			logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
		}
		logger = log.With(logger, "ts", log.DefaultTimestampUTC)
		logger = log.With(logger, "caller", log.DefaultCaller)
	}
	logger.Log("version", version)

	// Silence access errors logged internally by client-go
	k8slog := log.With(logger,
		"type", "internal kubernetes error",
		"kubernetes_caller", log.Valuer(func() interface{} {
			_, file, line, _ := runtime.Caller(5) // we want to log one level deeper than k8sruntime.HandleError
			idx := strings.Index(file, "/k8s.io/")
			return file[idx+1:] + ":" + strconv.Itoa(line)
		}))
	logErrorUnlessAccessRelated := func(err error) {
		errLower := strings.ToLower(err.Error())
		if k8serrors.IsForbidden(err) || k8serrors.IsNotFound(err) ||
			strings.Contains(errLower, "forbidden") ||
			strings.Contains(errLower, "not found") {
			return
		}
		k8slog.Log("err", err)
	}
	k8sruntime.ErrorHandlers = []func(error){logErrorUnlessAccessRelated}

	// Argument validation

	if config.GitReadonly {
		if config.SyncState == fluxsync.GitTagStateMode {
			logger.Log("warning", fmt.Sprintf("--git-readonly prevents use of --sync-state=%s. Forcing to --sync-state=%s", fluxsync.GitTagStateMode, fluxsync.NativeStateMode))
			config.SyncState = fluxsync.NativeStateMode
		}

		gitRelatedFlags := []string{
			"git-user",
			"git-email",
			"git-sync-tag",
			"git-set-author",
			"git-ci-skip",
			"git-ci-skip-message",
		}
		var changedGitRelatedFlags []string
		for _, gitRelatedFlag := range gitRelatedFlags {
			if fs.Changed(gitRelatedFlag) {
				changedGitRelatedFlags = append(changedGitRelatedFlags, gitRelatedFlag)
			}
		}
		if len(changedGitRelatedFlags) > 0 {
			logger.Log("warning", fmt.Sprintf("configuring any of {%s} has no effect when --git-readonly is set", strings.Join(changedGitRelatedFlags, ", ")))
		}
	}

	// Maintain backwards compatibility with the --registry-poll-interval
	// flag, but only if the --automation-interval is not set to a custom
	// (non default) value.
	if fs.Changed("registry-poll-interval") && !fs.Changed("automation-interval") {
		config.AutomationInterval = config.RegistryPollInterval
	}

	// Sort out values for the git tag and notes ref. There are
	// running deployments that assume the defaults as given, so don't
	// mess with those unless explicitly told.
	if fs.Changed("git-label") {
		config.GitSyncTag = config.GitLabel
		config.GitNotesRef = config.GitLabel
		for _, f := range []string{"git-sync-tag", "git-notes-ref"} {
			if fs.Changed(f) {
				logger.Log("overridden", f, "value", config.GitLabel)
			}
		}
	}

	if config.GitCISkipMessage == "" && config.GitCISkip {
		config.GitCISkipMessage = defaultGitSkipMessage
	}

	for _, path := range config.GitPath {
		if len(path) > 0 && path[0] == '/' {
			logger.Log("err", "subdirectory given as --git-path should not have leading forward slash")
			os.Exit(1)
		}
	}

	// Used to determine if we need to generate a SSH key and setup a keyring
	var httpGitURL bool
	if pURL, err := url.Parse(config.GitURL); err == nil {
		httpGitURL = pURL.Scheme == "http" || pURL.Scheme == "https"
	}

	if config.SSHKeygenDir == "" && !httpGitURL {
		logger.Log("info", fmt.Sprintf("SSH keygen dir (--ssh-keygen-dir) not provided, so using the deploy key volume (--k8s-secret-volume-mount-path=%s); this may cause problems if the deploy key volume is mounted read-only", config.K8sSecretVolumeMountPath))
		config.SSHKeygenDir = config.K8sSecretVolumeMountPath

	}

	// Import GPG keys, if we've been told where to look for them
	for _, p := range config.GitGPGKeyImport {
		keyfiles, err := gpg.ImportKeys(p, config.GitVerifySignatures)
		if err != nil {
			logger.Log("error", fmt.Sprintf("failed to import GPG key(s) from %s", p), "err", err.Error())
		}
		if keyfiles != nil {
			logger.Log("info", fmt.Sprintf("imported GPG key(s) from %s", p), "files", fmt.Sprintf("%v", keyfiles))
		}
	}

	possiblyRequired := stringset(RequireValues)
	for _, r := range config.RegistryRequire {
		if !possiblyRequired.has(r) {
			logger.Log("err", fmt.Sprintf("--registry-require value %q is not in possible values {%s}", r, strings.Join(RequireValues, ",")))
			os.Exit(1)
		}
	}
	mandatoryRegistry := stringset(config.RegistryRequire)

	if config.GitSecret && len(config.GitGPGKeyImport) == 0 {
		logger.Log("warning", fmt.Sprintf("--git-secret is enabled but there is no GPG key(s) provided using --git-gpg-key-import, we assume you mounted the keyring directly and continue"))
	}

	// Mechanical components.

	// When we can receive from this channel, it indicates that we
	// are ready to shut down.
	errc := make(chan error)
	// This signals other routines to shut down;
	shutdown := make(chan struct{})
	// .. and this is to wait for other routines to shut down cleanly.
	shutdownWg := &sync.WaitGroup{}

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errc <- fmt.Errorf("%s", <-c)
	}()

	// Cluster component.

	var restClientConfig *rest.Config
	{
		if config.K8sInCluster {
			logger.Log("msg", "using in cluster config to connect to the cluster")
			restClientConfig, err = rest.InClusterConfig()
			if err != nil {
				logger.Log("err", err)
				os.Exit(1)
			}
		} else {
			logger.Log("msg", fmt.Sprintf("using kube config: %q to connect to the cluster", *kubeConfig))
			restClientConfig, err = clientcmd.BuildConfigFromFlags("", *kubeConfig)
			if err != nil {
				logger.Log("err", err)
				os.Exit(1)
			}
		}
		restClientConfig.QPS = 50.0
		restClientConfig.Burst = 100
	}

	var clusterVersion string
	var sshKeyRing ssh.KeyRing
	var k8s cluster.Cluster
	var k8sManifests manifests.Manifests
	var imageCreds func() registry.ImageCreds
	{
		clientset, err := k8sclient.NewForConfig(restClientConfig)
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
		}
		dynamicClientset, err := k8sclientdynamic.NewForConfig(restClientConfig)
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
		}

		fhrClientset, err := hrclient.NewForConfig(restClientConfig)
		if err != nil {
			logger.Log("error", fmt.Sprintf("Error building hrclient clientset: %v", err))
			os.Exit(1)
		}

		hrClientset, err := helmopclient.NewForConfig(restClientConfig)
		if err != nil {
			logger.Log("error", fmt.Sprintf("Error building helm operator clientset: %v", err))
			os.Exit(1)
		}

		crdClient, err := crd.NewForConfig(restClientConfig)
		if err != nil {
			logger.Log("error", fmt.Sprintf("Error building API extensions (CRD) clientset: %v", err))
			os.Exit(1)
		}
		discoClientset := kubernetes.MakeCachedDiscovery(clientset.Discovery(), crdClient, shutdown)

		serverVersion, err := clientset.ServerVersion()
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
		}
		clusterVersion = "kubernetes-" + serverVersion.GitVersion

		if config.K8sInCluster && !httpGitURL {
			namespace, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
			if err != nil {
				logger.Log("err", err)
				os.Exit(1)
			}

			sshKeyRing, err = kubernetes.NewSSHKeyRing(kubernetes.SSHKeyRingConfig{
				SecretAPI:             clientset.CoreV1().Secrets(string(namespace)),
				SecretName:            config.K8sSecretName,
				SecretVolumeMountPath: config.K8sSecretVolumeMountPath,
				SecretDataKey:         config.K8sSecretDataKey,
				KeyBits:               sshKeyBits,
				KeyType:               sshKeyType,
				KeyGenDir:             config.SSHKeygenDir,
			})
			if err != nil {
				logger.Log("err", err)
				os.Exit(1)
			}

			publicKey, privateKeyPath := sshKeyRing.KeyPair()

			logger := log.With(logger, "component", "cluster")
			logger.Log("identity", privateKeyPath)
			logger.Log("identity.pub", strings.TrimSpace(publicKey.Key))
		} else {
			sshKeyRing = ssh.NewNopSSHKeyRing()
		}

		logger.Log("host", restClientConfig.Host, "version", clusterVersion)

		kubectl := config.KubernetesKubectl
		if kubectl == "" {
			kubectl, err = exec.LookPath("kubectl")
		} else {
			_, err = os.Stat(kubectl)
		}
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
		}
		logger.Log("kubectl", kubectl)

		client := kubernetes.MakeClusterClientset(clientset, dynamicClientset, fhrClientset, hrClientset, discoClientset)
		kubectlApplier := kubernetes.NewKubectl(kubectl, restClientConfig)
		allowedNamespaces := append(config.K8sNamespaceWhitelist, config.K8sAllowNamespace...)
		k8sInst := kubernetes.NewCluster(client, kubectlApplier, sshKeyRing, logger, allowedNamespaces, config.RegistryExcludeImage)
		k8sInst.GC = config.SyncGarbageCollection
		k8sInst.DryGC = config.SyncGarbageCollectionDry

		if err := k8sInst.Ping(); err != nil {
			logger.Log("ping", err)
		} else {
			logger.Log("ping", true)
		}

		k8s = k8sInst
		imageCreds = k8sInst.ImagesToFetch
		// There is only one way we currently interpret a repo of
		// files as manifests, and that's as Kubernetes yamels.
		namespacer, err := kubernetes.NewNamespacer(discoClientset)
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
		}
		k8sManifests = kubernetes.NewManifests(namespacer, logger)
	}

	// Wrap the procedure for collecting images to scan
	{
		awsConf := registry.AWSRegistryConfig{
			Regions:    config.RegistryECRRegion,
			AccountIDs: config.RegistryECRIncludeID,
			BlockIDs:   config.RegistryECRExcludeID,
		}

		awsPreflight, credsWithAWSAuth := registry.ImageCredsWithAWSAuth(imageCreds, log.With(logger, "component", "aws"), awsConf)
		if mandatoryRegistry.has(RequireECR) {
			if err := awsPreflight(); err != nil {
				logger.Log("error", "AWS API required (due to --registry-require=ecr), but not available", "err", err)
				os.Exit(1)
			}
		}
		imageCreds = credsWithAWSAuth

		if config.DockerConfig != "" {
			credsWithDefaults, err := registry.ImageCredsWithDefaults(imageCreds, config.DockerConfig)
			if err != nil {
				logger.Log("warning", "--docker-config not used; pre-flight check failed", "err", err)
			} else {
				imageCreds = credsWithDefaults
			}
		}
	}

	// Registry components
	var cacheRegistry registry.Registry
	var cacheWarmer *cache.Warmer
	{
		// Cache client, for use by registry and cache warmer
		var cacheClient cache.Client
		var memcacheClient *registryMemcache.MemcacheClient
		memcacheConfig := registryMemcache.MemcacheConfig{
			Host:           config.MemcachedHostname,
			Service:        config.MemcachedService,
			Timeout:        config.MemcachedTimeout,
			UpdateInterval: 1 * time.Minute,
			Logger:         log.With(logger, "component", "memcached"),
			MaxIdleConns:   config.RegistryBurst,
		}

		// if no memcached service is specified use the ClusterIP name instead of SRV records
		if config.MemcachedService == "" {
			memcacheClient = registryMemcache.NewFixedServerMemcacheClient(memcacheConfig,
				fmt.Sprintf("%s:%d", config.MemcachedHostname, config.MemcachedPort))
		} else {
			memcacheClient = registryMemcache.NewMemcacheClient(memcacheConfig)
		}

		defer memcacheClient.Stop()
		cacheClient = cache.InstrumentClient(memcacheClient)

		cacheRegistry = &cache.Cache{
			Reader: cacheClient,
			Decorators: []cache.Decorator{
				cache.TimestampLabelWhitelist(config.RegistryUseLabels),
			},
		}
		cacheRegistry = registry.NewInstrumentedRegistry(cacheRegistry)

		// Remote client, for warmer to refresh entries
		registryLogger := log.With(logger, "component", "registry")
		registryLimits := &registryMiddleware.RateLimiters{
			RPS:    config.RegistryRPS,
			Burst:  config.RegistryBurst,
			Logger: log.With(logger, "component", "ratelimiter"),
		}
		remoteFactory := &registry.RemoteClientFactory{
			Logger:        registryLogger,
			Limiters:      registryLimits,
			Trace:         config.RegistryTrace,
			InsecureHosts: config.RegistryInsecureHost,
		}

		// Warmer
		var err error
		cacheWarmer, err = cache.NewWarmer(remoteFactory, cacheClient, config.RegistryBurst)
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
		}
	}

	// Checkpoint: we want to include the fact of whether the daemon
	// was given a Git repo it could clone; but the expected scenario
	// is that it will have been set up already, and we don't want to
	// report anything before seeing if it works. So, don't start
	// until we have failed or succeeded.
	updateCheckLogger := log.With(logger, "component", "checkpoint")
	checkpointFlags := map[string]string{
		"cluster-version": clusterVersion,
		"git-configured":  strconv.FormatBool(config.GitURL != ""),
	}
	checkpoint.CheckForUpdates(product, version, checkpointFlags, updateCheckLogger)

	gitRemote := git.Remote{URL: config.GitURL}
	gitConfig := git.Config{
		Paths:       config.GitPath,
		Branch:      config.GitBranch,
		NotesRef:    config.GitNotesRef,
		UserName:    config.GitUser,
		UserEmail:   config.GitEmail,
		SigningKey:  config.GitSigningKey,
		SetAuthor:   config.GitSetAuthor,
		SkipMessage: config.GitCISkipMessage,
	}

	repo := git.NewRepo(gitRemote, git.PollInterval(config.GitPollInterval), git.Timeout(config.GitTimeout), git.Branch(config.GitBranch), git.IsReadOnly(config.GitReadonly))
	{
		shutdownWg.Add(1)
		go func() {
			err := repo.Start(shutdown, shutdownWg)
			if err != nil {
				errc <- err
			}
		}()
	}

	logger.Log(
		"url", config.GitURL,
		"user", config.GitUser,
		"email", config.GitEmail,
		"signing-key", config.GitSigningKey,
		"verify-signatures", config.GitVerifySignatures,
		"sync-tag", config.GitSyncTag,
		"state", config.SyncState,
		"readonly", config.GitReadonly,
		"notes-ref", config.GitNotesRef,
		"set-author", config.GitSetAuthor,
		"git-secret", config.GitSecret,
	)

	var jobs *job.Queue
	{
		jobs = job.NewQueue(shutdown, shutdownWg)
	}

	var syncProvider fluxsync.State
	switch config.SyncState {
	case fluxsync.NativeStateMode:
		namespace, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
		}

		syncProvider, err = fluxsync.NewNativeSyncProvider(
			string(namespace),
			config.K8sSecretName,
		)
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
		}

	case fluxsync.GitTagStateMode:
		syncProvider, err = fluxsync.NewGitTagSyncProvider(
			repo,
			config.GitSyncTag,
			config.GitSigningKey,
			config.GitVerifySignatures,
			gitConfig,
		)
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
		}

	default:
		logger.Log("error", "unknown sync state mode", "mode", config.SyncState)
		os.Exit(1)
	}

	daemon := &daemon.Daemon{
		V:                         version,
		Cluster:                   k8s,
		Manifests:                 k8sManifests,
		Registry:                  cacheRegistry,
		ImageRefresh:              make(chan image.Name, 100), // size chosen by fair dice roll
		Repo:                      repo,
		GitConfig:                 gitConfig,
		Jobs:                      jobs,
		JobStatusCache:            &job.StatusCache{Size: 100},
		Logger:                    log.With(logger, "component", "daemon"),
		ManifestGenerationEnabled: config.ManifestGeneration,
		GitSecretEnabled:          config.GitSecret,
		LoopVars: &daemon.LoopVars{
			SyncInterval:        config.SyncInterval,
			SyncTimeout:         config.SyncTimeout,
			SyncState:           syncProvider,
			AutomationInterval:  config.AutomationInterval,
			GitTimeout:          config.GitTimeout,
			GitVerifySignatures: config.GitVerifySignatures,
		},
	}

	{
		// Connect to fluxsvc if given an upstream address
		if config.Connect != "" {
			upstreamLogger := log.With(logger, "component", "upstream")
			upstreamLogger.Log("URL", config.Connect)
			upstream, err := daemonhttp.NewUpstream(
				&http.Client{Timeout: 10 * time.Second},
				fmt.Sprintf("fluxd/%v", version),
				client.Token(config.Token),
				transport.NewUpstreamRouter(),
				config.Connect,
				remote.NewErrorLoggingServer(daemon, upstreamLogger),
				config.RPCTimeout,
				upstreamLogger,
			)
			if err != nil {
				logger.Log("err", err)
				os.Exit(1)
			}
			daemon.EventWriter = upstream
			go func() {
				<-shutdown
				upstream.Close()
			}()
		} else {
			logger.Log("upstream", "no upstream URL given")
		}
	}

	shutdownWg.Add(1)
	go daemon.Loop(shutdown, shutdownWg, log.With(logger, "component", "sync-loop"))

	cacheWarmer.Notify = daemon.AskForAutomatedWorkloadImageUpdates
	cacheWarmer.Priority = daemon.ImageRefresh
	cacheWarmer.Trace = config.RegistryTrace
	shutdownWg.Add(1)
	go cacheWarmer.Loop(log.With(logger, "component", "warmer"), shutdown, shutdownWg, imageCreds)

	go func() {
		mux := http.DefaultServeMux
		// Serve /metrics alongside API
		if config.ListenMetrics == "" {
			mux.Handle("/metrics", promhttp.Handler())
		}
		handler := daemonhttp.NewHandler(daemon, daemonhttp.NewRouter())
		mux.Handle("/api/flux/", http.StripPrefix("/api/flux", handler))
		logger.Log("addr", config.Listen)
		errc <- http.ListenAndServe(config.Listen, mux)
	}()

	if config.ListenMetrics != "" {
		go func() {
			mux := http.NewServeMux()
			mux.Handle("/metrics", promhttp.Handler())
			logger.Log("metrics-addr", config.ListenMetrics)
			errc <- http.ListenAndServe(config.ListenMetrics, mux)
		}()
	}

	// wait here until stopping.
	logger.Log("exiting", <-errc)
	close(shutdown)
	shutdownWg.Wait()
}

func homeDir() string {
	// nix
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	// windows
	if h := os.Getenv("USERPROFILE"); h != "" {
		return h
	}
	return ""
}
