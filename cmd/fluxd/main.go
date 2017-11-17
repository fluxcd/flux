package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
	"github.com/weaveworks/go-checkpoint"
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"context"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/cluster/kubernetes"
	"github.com/weaveworks/flux/daemon"
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/flux/git"
	transport "github.com/weaveworks/flux/http"
	daemonhttp "github.com/weaveworks/flux/http/daemon"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/registry"
	registryMemcache "github.com/weaveworks/flux/registry/cache"
	registryMiddleware "github.com/weaveworks/flux/registry/middleware"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/ssh"
)

var version string

const (
	// The number of connections chosen for memcache and remote GETs should match for best performance (hence the single hardcoded value)
	// Value chosen through performance tests on sock-shop. I was unable to get higher performance than this.
	defaultRemoteConnections   = 125 // Chosen performance tests on sock-shop. Unable to get higher performance than this.
	defaultMemcacheConnections = 10  // This doesn't need to be high. The user is only requesting one tag/image at a time.

	// There are running systems that assume these defaults (by not
	// supplying a value for one or both). Don't change them.
	defaultGitSyncTag  = "flux-sync"
	defaultGitNotesRef = "flux"
)

func optionalVar(fs *pflag.FlagSet, value ssh.OptionalValue, name, usage string) ssh.OptionalValue {
	fs.Var(value, name, usage)
	return value
}

func main() {
	// Flag domain.
	fs := pflag.NewFlagSet("default", pflag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "DESCRIPTION\n")
		fmt.Fprintf(os.Stderr, "  fluxd is the agent of flux.\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "FLAGS\n")
		fs.PrintDefaults()
	}
	// This mirrors how kubectl extracts information from the environment.
	var (
		listenAddr        = fs.StringP("listen", "l", ":3030", "Listen address where /metrics and API will be served")
		kubernetesKubectl = fs.String("kubernetes-kubectl", "", "Optional, explicit path to kubectl tool")
		versionFlag       = fs.Bool("version", false, "Get version number")
		// Git repo & key etc.
		gitURL       = fs.String("git-url", "", "URL of git repo with Kubernetes manifests; e.g., git@github.com:weaveworks/flux-example")
		gitBranch    = fs.String("git-branch", "master", "branch of git repo to use for Kubernetes manifests")
		gitPath      = fs.String("git-path", "", "path within git repo to locate Kubernetes manifests (relative path)")
		gitUser      = fs.String("git-user", "Weave Flux", "username to use as git committer")
		gitEmail     = fs.String("git-email", "support@weave.works", "email to use as git committer")
		gitSetAuthor = fs.Bool("git-set-author", false, "If set, the author of git commits will reflect the user who initiated the commit and will differ from the git committer.")
		gitLabel     = fs.String("git-label", "", "label to keep track of sync progress; overrides both --git-sync-tag and --git-notes-ref")
		// Old git config; still used if --git-label is not supplied, but --git-label is preferred.
		gitSyncTag  = fs.String("git-sync-tag", defaultGitSyncTag, "tag to use to mark sync progress for this cluster")
		gitNotesRef = fs.String("git-notes-ref", defaultGitNotesRef, "ref to use for keeping commit annotations in git notes")

		gitPollInterval = fs.Duration("git-poll-interval", 5*time.Minute, "period at which to poll git repo for new commits")
		// registry
		memcachedHostname    = fs.String("memcached-hostname", "", "Hostname for memcached service to use when caching chunks. If empty, no memcached will be used.")
		memcachedTimeout     = fs.Duration("memcached-timeout", time.Second, "Maximum time to wait before giving up on memcached requests.")
		memcachedService     = fs.String("memcached-service", "memcached", "SRV service used to discover memcache servers.")
		registryCacheExpiry  = fs.Duration("registry-cache-expiry", 20*time.Minute, "Duration to keep cached registry tag info. Must be < 1 month.")
		registryPollInterval = fs.Duration("registry-poll-interval", 5*time.Minute, "period at which to poll registry for new images")
		registryRPS          = fs.Int("registry-rps", 200, "maximum registry requests per second per host")
		registryBurst        = fs.Int("registry-burst", defaultRemoteConnections, "maximum number of warmer connections to remote and memcache")

		// k8s-secret backed ssh keyring configuration
		k8sSecretName            = fs.String("k8s-secret-name", "flux-git-deploy", "Name of the k8s secret used to store the private SSH key")
		k8sSecretVolumeMountPath = fs.String("k8s-secret-volume-mount-path", "/etc/fluxd/ssh", "Mount location of the k8s secret storing the private SSH key")
		k8sSecretDataKey         = fs.String("k8s-secret-data-key", "identity", "Data key holding the private SSH key within the k8s secret")
		// SSH key generation
		sshKeyBits = optionalVar(fs, &ssh.KeyBitsValue{}, "ssh-keygen-bits", "-b argument to ssh-keygen (default unspecified)")
		sshKeyType = optionalVar(fs, &ssh.KeyTypeValue{}, "ssh-keygen-type", "-t argument to ssh-keygen (default unspecified)")

		upstreamURL = fs.String("connect", "", "Connect to an upstream service e.g., Weave Cloud, at this base address")
		token       = fs.String("token", "", "Authentication token for upstream service")

		// Deprecated
		_ = fs.String("docker-config", "", "path to a docker config to use for credentials")
	)

	fs.MarkDeprecated("docker-config", "credentials are taken from imagePullSecrets now")

	fs.Parse(os.Args)

	if version == "" {
		version = "unversioned"
	}
	if *versionFlag {
		fmt.Println(version)
		os.Exit(0)
	}

	// Logger component.
	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.With(logger, "ts", log.DefaultTimestampUTC)
		logger = log.With(logger, "caller", log.DefaultCaller)
	}

	// Sort out values for the git tag and notes ref. There are
	// running deployments that assume the defaults as given, so don't
	// mess with those unless explicitly told.
	if fs.Changed("git-label") {
		*gitSyncTag = *gitLabel
		*gitNotesRef = *gitLabel
		for _, f := range []string{"git-sync-tag", "git-notes-ref"} {
			if fs.Changed(f) {
				logger.Log("overridden", f, "value", *gitLabel)
			}
		}
	}

	// Platform component.
	var clusterVersion string
	var sshKeyRing ssh.KeyRing
	var k8s cluster.Cluster
	var image_creds func() registry.ImageCreds
	var k8sManifests cluster.Manifests
	{
		restClientConfig, err := rest.InClusterConfig()
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
		}

		restClientConfig.QPS = 50.0
		restClientConfig.Burst = 100

		clientset, err := k8sclient.NewForConfig(restClientConfig)
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
		}

		serverVersion, err := clientset.ServerVersion()
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
		}
		clusterVersion = "kubernetes-" + serverVersion.GitVersion

		namespace, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
		}

		sshKeyRing, err = kubernetes.NewSSHKeyRing(kubernetes.SSHKeyRingConfig{
			SecretAPI:             clientset.Core().Secrets(string(namespace)),
			SecretName:            *k8sSecretName,
			SecretVolumeMountPath: *k8sSecretVolumeMountPath,
			SecretDataKey:         *k8sSecretDataKey,
			KeyBits:               sshKeyBits,
			KeyType:               sshKeyType,
		})
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
		}

		publicKey, privateKeyPath := sshKeyRing.KeyPair()

		logger := log.With(logger, "component", "platform")
		logger.Log("identity", privateKeyPath)
		logger.Log("identity.pub", publicKey.Key)
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

		kubectlApplier := kubernetes.NewKubectl(kubectl, restClientConfig, os.Stdout, os.Stderr)
		k8s_inst, err := kubernetes.NewCluster(clientset, kubectlApplier, sshKeyRing, logger)
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
		}

		if err := k8s_inst.Ping(); err != nil {
			logger.Log("ping", err)
		} else {
			logger.Log("ping", true)
		}

		image_creds = k8s_inst.ImagesToFetch
		k8s = k8s_inst
		// There is only one way we currently interpret a repo of
		// files as manifests, and that's as Kubernetes yamels.
		k8sManifests = &kubernetes.Manifests{}
	}

	// Registry components
	var cache registry.Registry
	var cacheWarmer registry.Warmer
	{
		// Cache
		var memcacheRegistry registryMemcache.Client
		if *memcachedHostname != "" {
			memcacheRegistry = registryMemcache.NewMemcacheClient(registryMemcache.MemcacheConfig{
				Host:           *memcachedHostname,
				Service:        *memcachedService,
				Timeout:        *memcachedTimeout,
				UpdateInterval: 1 * time.Minute,
				Logger:         log.With(logger, "component", "memcached"),
				MaxIdleConns:   defaultMemcacheConnections,
			})
			memcacheRegistry = registryMemcache.InstrumentMemcacheClient(memcacheRegistry)
			defer memcacheRegistry.Stop()
		}
		var memcacheWarmer registryMemcache.Client
		if *memcachedHostname != "" {
			memcacheWarmer = registryMemcache.NewMemcacheClient(registryMemcache.MemcacheConfig{
				Host:           *memcachedHostname,
				Service:        *memcachedService,
				Timeout:        *memcachedTimeout,
				UpdateInterval: 1 * time.Minute,
				Logger:         log.With(logger, "component", "memcached"),
				MaxIdleConns:   *registryBurst,
			})
			memcacheWarmer = registryMemcache.InstrumentMemcacheClient(memcacheWarmer)
			defer memcacheWarmer.Stop()
		}

		cacheLogger := log.With(logger, "component", "cache")
		cache = &registry.ClientRegistry{
			Factory: registry.NewCacheClientFactory(cacheLogger, memcacheRegistry, *registryCacheExpiry),
			Logger:  cacheLogger,
		}
		cache = registry.NewInstrumentedRegistry(cache)

		// Remote
		registryLogger := log.With(logger, "component", "registry")
		registryLimits := &registryMiddleware.RateLimiters{
			RPS:   *registryRPS,
			Burst: *registryBurst,
		}
		remoteFactory := &registry.RemoteClientFactory{
			Logger:   registryLogger,
			Limiters: registryLimits,
		}

		// Warmer
		warmerLogger := log.With(logger, "component", "warmer")
		cacheWarmer = registry.Warmer{
			Logger:        warmerLogger,
			ClientFactory: remoteFactory,
			Expiry:        *registryCacheExpiry,
			Reader:        memcacheWarmer,
			Writer:        memcacheWarmer,
			Burst:         *registryBurst,
		}
	}

	gitRemoteConfig, err := flux.NewGitRemoteConfig(*gitURL, *gitBranch, *gitPath)
	if err != nil {
		logger.Log("err", err)
		os.Exit(1)
	}
	// Indirect reference to a daemon, initially of the NotReady variety
	notReadyDaemon := daemon.NewNotReadyDaemon(
		version, k8s, gitRemoteConfig, errors.New("waiting to clone repo"))

	daemonRef := daemon.NewRef(notReadyDaemon)

	var eventWriter event.EventWriter
	{
		// Connect to fluxsvc if given an upstream address
		if *upstreamURL != "" {
			upstreamLogger := log.With(logger, "component", "upstream")
			upstreamLogger.Log("URL", *upstreamURL)
			upstream, err := daemonhttp.NewUpstream(
				&http.Client{Timeout: 10 * time.Second},
				fmt.Sprintf("fluxd/%v", version),
				flux.Token(*token),
				transport.NewUpstreamRouter(),
				*upstreamURL,
				&remote.ErrorLoggingPlatform{daemonRef, upstreamLogger},
				upstreamLogger,
			)
			if err != nil {
				logger.Log("err", err)
				os.Exit(1)
			}
			eventWriter = upstream
			defer upstream.Close()
		} else {
			logger.Log("upstream", "no upstream URL given")
		}
	}

	// Mechanical components.
	errc := make(chan error)
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errc <- fmt.Errorf("%s", <-c)
	}()

	// HTTP transport component, for metrics
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		handler := daemonhttp.NewHandler(daemonRef, daemonhttp.NewRouter())
		mux.Handle("/api/flux/", http.StripPrefix("/api/flux", handler))
		logger.Log("addr", *listenAddr)
		errc <- http.ListenAndServe(*listenAddr, mux)
	}()

	// Checkpoint: we want to include the fact of whether the daemon
	// was given a Git repo it could clone; but the expected scenario
	// is that it will have been set up already, and we don't want to
	// report anything before seeing if it works. So, don't start
	// until we have failed or succeeded.
	var checker *checkpoint.Checker
	updateCheckLogger := log.With(logger, "component", "checkpoint")

	var repo git.Repo
	var checkout *git.Checkout
	{
		repo = git.Repo{
			GitRemoteConfig: gitRemoteConfig,
			KeyRing:         sshKeyRing,
		}
		gitConfig := git.Config{
			SyncTag:   *gitSyncTag,
			NotesRef:  *gitNotesRef,
			UserName:  *gitUser,
			UserEmail: *gitEmail,
			SetAuthor: *gitSetAuthor,
		}

		for checkout == nil {
			ctx, cancel := context.WithTimeout(context.Background(), git.DefaultCloneTimeout)
			working, err := repo.Clone(ctx, gitConfig)
			cancel()
			if err != nil {
				if checker == nil {
					checker = checkForUpdates(clusterVersion, "false", updateCheckLogger)
				}
				logger.Log("component", "git", "err", err.Error())
				notReadyDaemon.UpdateReason(err)
				time.Sleep(10 * time.Second)
			} else {
				if checker != nil {
					checker.Stop()
				}
				checker = checkForUpdates(clusterVersion, "true", updateCheckLogger)
				logger.Log("working-dir", working.Dir,
					"user", *gitUser,
					"email", *gitEmail,
					"sync-tag", *gitSyncTag,
					"notes-ref", *gitNotesRef,
					"set-author", *gitSetAuthor)
				checkout = working
			}
		}
	}

	shutdown := make(chan struct{})
	shutdownWg := &sync.WaitGroup{}

	var jobs *job.Queue
	{
		jobs = job.NewQueue(shutdown, shutdownWg)
	}

	daemon := &daemon.Daemon{
		V:            version,
		Cluster:      k8s,
		Manifests:    k8sManifests,
		Registry:     cache,
		ImageRefresh: make(chan image.Name, 100), // size chosen by fair dice roll
		Repo:         repo, Checkout: checkout,
		Jobs:           jobs,
		JobStatusCache: &job.StatusCache{Size: 100},

		EventWriter: eventWriter,
		Logger:      log.With(logger, "component", "daemon"), LoopVars: &daemon.LoopVars{
			GitPollInterval:      *gitPollInterval,
			RegistryPollInterval: *registryPollInterval,
		},
	}

	shutdownWg.Add(1)
	go daemon.GitPollLoop(shutdown, shutdownWg, log.With(logger, "component", "sync-loop"))

	cacheWarmer.Notify = daemon.AskForImagePoll
	cacheWarmer.Priority = daemon.ImageRefresh
	shutdownWg.Add(1)
	go cacheWarmer.Loop(shutdown, shutdownWg, image_creds)

	// Update daemonRef so that upstream and handlers point to fully working daemon
	daemonRef.UpdatePlatform(daemon)

	// Go!
	logger.Log("exiting", <-errc)
	close(shutdown)
	shutdownWg.Wait()
}
