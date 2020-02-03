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

	helmopclient "github.com/fluxcd/helm-operator/pkg/client/clientset/versioned"

	hrclient "github.com/fluxcd/flux/integrations/client/clientset/versioned"
	"github.com/fluxcd/flux/pkg/checkpoint"
	"github.com/fluxcd/flux/pkg/cluster"
	"github.com/fluxcd/flux/pkg/cluster/kubernetes"
	"github.com/fluxcd/flux/pkg/config"
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

	k8sInClusterSecretsBaseDir = "/var/run/secrets/kubernetes.io"
)

var (
	RequireValues = []string{RequireECR}
)

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
	// --- Flags ---

	// For most things we want to allow configuration to be supplied
	// in a config file OR via command-line flags. For some things
	// this does not make sense (e.g., --version), and for other
	// things we need to manipulate the flag definitions before
	// subjecting to parsing. So initialising and parsing flags takes
	// a few phases.

	fs := pflag.NewFlagSet("default", pflag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "DESCRIPTION\n")
		fmt.Fprintf(os.Stderr, "  fluxd is the agent of flux.\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "FLAGS\n")
		fs.PrintDefaults()
	}

	// Define all the flags can come from the config file too. They
	// don't get assigned to vars because we'll be putting them all
	// into a config.Config struct, via viper.BindPFlag, and
	// consulting the struct.
	defineConfigFlags(fs, func(err error) {
		fmt.Fprintf(os.Stderr, "Error: failed to initialise flags: %s\n", err.Error())
		os.Exit(2)
	})

	// --- These _don't_ come from a config file or get put in the Config struct
	var (
		versionFlag = fs.Bool("version", false, "get version number")

		// not present in the config struct, since "extra for experts" or only relevant when running locally
		kubernetesKubectl = fs.String("kubernetes-kubectl", "", "optional, explicit path to kubectl tool")
		// SSH key generation
		sshKeyBits = optionalVar(fs, &ssh.KeyBitsValue{}, "ssh-keygen-bits", "-b argument to ssh-keygen (default unspecified)")
		sshKeyType = optionalVar(fs, &ssh.KeyTypeValue{}, "ssh-keygen-type", "-t argument to ssh-keygen (default unspecified)")

		// not present in the config struct, and ignored, but accepted for backward-compatibility
		_ = fs.Bool("k8s-in-cluster", true, "set this to false if fluxd is NOT deployed as a container inside Kubernetes")
		_ = fs.Duration("registry-cache-expiry", 0, "")

		// not present in the config struct, but accepted and taken into account, for backward-compatibility
		registryPollInterval  = fs.Duration("registry-poll-interval", 5*time.Minute, "period at which to check for updated images")
		k8sNamespaceWhitelist = fs.StringSlice("k8s-namespace-whitelist", []string{}, "experimental, optional: restrict the view of the cluster to the namespaces listed. All namespaces are included if this is not set")
	)

	fs.MarkDeprecated("registry-cache-expiry", "no longer used; cache entries are expired adaptively according to how often they change")
	fs.MarkDeprecated("k8s-namespace-whitelist", "changed to --k8s-allow-namespace, use that instead")
	fs.MarkDeprecated("registry-poll-interval", "changed to --automation-interval, use that instead")
	fs.MarkDeprecated("k8s-in-cluster", "no longer used")

	// Support --kube-config for backward compatibility, but otherwise
	// let the K8s client code / kubectl pick up KUBECONFIG from the
	// environment.
	var kubeConfig *string
	{
		// The default for kubeconfig depends on whether $HOME (or the equivalent) is set
		if home := homeDir(); home != "" {
			kubeConfig = fs.String("kube-config", filepath.Join(home, ".kube", "config"), "the absolute path of the k8s config file.")
		} else {
			kubeConfig = fs.String("kube-config", "", "the absolute path of the k8s config file.")
		}
	}
	fs.MarkDeprecated("kube-config", "please use the KUBECONFIG environment variable instead")

	// Parse so we can check if any arguments that were passed on the
	// command-line are valid, and exit early.

	err := fs.Parse(os.Args[1:])
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

	viper.SetConfigName(config.ConfigName)
	viper.AddConfigPath(config.ConfigPath)
	viper.SetConfigType(config.ConfigType)

	// If there's no config file, fine. If there IS a config file, but it's garbage, we need to exit(>0).
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Fprintf(os.Stderr, "Info: config file not found: %s\n", err.Error())
		} else {
			fmt.Fprintf(os.Stderr, "Error: found config file at %s but failed to load it: %s\n", viper.ConfigFileUsed(), err.Error())
			os.Exit(2)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Error: using configuration at %s, with command-line overrides\n", viper.ConfigFileUsed())
	}

	var config config.Config
	if err = viper.Unmarshal(&config); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to initialise config: %s\n", err.Error())
		os.Exit(2)
	}

	if err = config.IsValid(); viper.ConfigFileUsed() != "" && err != nil {
		fmt.Fprintf(os.Stderr, "found config file at %s but it is not valid: %s\n", viper.ConfigFileUsed(), err.Error())
		os.Exit(2)
	}

	// --- From this point, we can just consult the config struct for values ---

	// viper.IsSet, which we could otherwise use for determining
	// whether a flag has been supplied, is broken:
	// https://github.com/spf13/viper/pull/331. So we have to proceed
	// by other means.
	isSet := func(flag string) bool {
		return fs.Changed(flag) || viper.InConfig(flag)
	}

	// Explicitly initialize klog to enable stderr logging,
	// and parse our own flags.
	klogFlags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(klogFlags)

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
			if isSet(gitRelatedFlag) {
				changedGitRelatedFlags = append(changedGitRelatedFlags, gitRelatedFlag)
			}
		}
		if len(changedGitRelatedFlags) > 0 {
			logger.Log("warning", fmt.Sprintf("configuring any of {%s} has no effect when --git-readonly is set", strings.Join(changedGitRelatedFlags, ", ")))
		}
	}

	// Maintain backwards compatibility with the --registry-poll-interval
	// _flag_, but only if the --automation-interval is not set to a custom
	// (non default) value anywhere.
	if fs.Changed("registry-poll-interval") && !isSet("automation-interval") {
		config.AutomationInterval = *registryPollInterval
	}

	// Sort out values for the git tag and notes ref. There are
	// running deployments that assume the defaults as given, so don't
	// mess with those unless explicitly told.
	if isSet("git-label") {
		config.GitSyncTag = config.GitLabel
		config.GitNotesRef = config.GitLabel
		for _, f := range []string{"git-sync-tag", "git-notes-ref"} {
			if isSet(f) {
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
	// --- Here ends all the flag parsing and validation ---

	if config.SopsEnabled && len(config.GitGPGKeyImport) == 0 {
		logger.Log("warning", fmt.Sprintf("--sops is enabled but there is no GPG key(s) provided using --git-gpg-key-import, we assume that the means of decryption has been provided in another way"))
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
		var err error
		if *kubeConfig != "" {
			logger.Log("msg", fmt.Sprintf("using kube config: %q to connect to the cluster", *kubeConfig))
			restClientConfig, err = clientcmd.BuildConfigFromFlags("", *kubeConfig)
		} else {
			restClientConfig, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
				clientcmd.NewDefaultClientConfigLoadingRules(),
				&clientcmd.ConfigOverrides{},
			).ClientConfig()
		}
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
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

		fileinfo, err := os.Stat(k8sInClusterSecretsBaseDir)
		isInCluster := err == nil && fileinfo.IsDir()
		if isInCluster && !httpGitURL {
			namespace, err := ioutil.ReadFile(filepath.Join(k8sInClusterSecretsBaseDir, "serviceaccount/namespace"))
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

		kubectl := *kubernetesKubectl
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

		allowedNamespaces := make(map[string]struct{})
		for _, n := range append(*k8sNamespaceWhitelist, config.K8sAllowNamespace...) {
			allowedNamespaces[n] = struct{}{}
		}
		k8sInst := kubernetes.NewCluster(client, kubectlApplier, sshKeyRing, logger, allowedNamespaces, config.RegistryExcludeImage, config.K8sExcludeResource)
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
		namespacer, err := kubernetes.NewNamespacer(discoClientset, config.K8sDefaultNamespace)
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
		}
		if config.SopsEnabled {
			k8sManifests = kubernetes.NewSopsManifests(namespacer, logger)
		} else {
			k8sManifests = kubernetes.NewManifests(namespacer, logger)
		}
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
	var imageRegistry registry.Registry = registry.ImageScanDisabledRegistry{}
	var cacheWarmer *cache.Warmer
	if !config.RegistryDisableScanning {
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

		imageRegistry = &cache.Cache{
			Reader: cacheClient,
			Decorators: []cache.Decorator{
				cache.TimestampLabelWhitelist(config.RegistryUseLabels),
			},
		}
		imageRegistry = registry.NewInstrumentedRegistry(imageRegistry)

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
		"url", gitRemote.SafeURL(),
		"user", config.GitUser,
		"email", config.GitEmail,
		"signing-key", config.GitSigningKey,
		"verify-signatures", config.GitVerifySignatures,
		"sync-tag", config.GitSyncTag,
		"state", config.SyncState,
		"readonly", config.GitReadonly,
		"registry-disable-scanning", config.RegistryDisableScanning,
		"notes-ref", config.GitNotesRef,
		"set-author", config.GitSetAuthor,
		"git-secret", config.GitSecret,
		"sops", config.SopsEnabled,
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
		Registry:                  imageRegistry,
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
			ImageScanDisabled:   config.RegistryDisableScanning,
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

	if config.RegistryDisableScanning {
		cacheWarmer.Notify = daemon.AskForAutomatedWorkloadImageUpdates
		cacheWarmer.Priority = daemon.ImageRefresh
		cacheWarmer.Trace = config.RegistryTrace
		shutdownWg.Add(1)
		go cacheWarmer.Loop(log.With(logger, "component", "warmer"), shutdown, shutdownWg, imageCreds)
	}

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
