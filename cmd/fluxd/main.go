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
	k8sclient "k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/rest"

	//	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/cluster/kubernetes"
	"github.com/weaveworks/flux/daemon"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/history"
	transport "github.com/weaveworks/flux/http"
	daemonhttp "github.com/weaveworks/flux/http/daemon"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/registry"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/ssh"
)

var version string

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
		gitURL          = fs.String("git-url", "", "URL of git repo with Kubernetes manifests; e.g., git@github.com:weaveworks/flux-example")
		gitBranch       = fs.String("git-branch", "master", "branch of git repo to use for Kubernetes manifests")
		gitPath         = fs.String("git-path", "", "path within git repo to locate Kubernetes manifests")
		gitUser         = fs.String("git-user", "Weave Flux", "username to use as git committer")
		gitEmail        = fs.String("git-email", "support@weave.works", "email to use as git committer")
		gitSyncTag      = fs.String("git-sync-tag", "flux-sync", "tag to use to mark sync progress for this cluster")
		gitNotesRef     = fs.String("git-notes-ref", "flux", "ref to use for keeping commit annotations in git notes")
		gitPollInterval = fs.Duration("git-poll-interval", 5*time.Minute, "period at which to poll git repo for new commits")
		// registry
		dockerCredFile       = fs.String("docker-config", "~/.docker/config.json", "Path to config file with credentials for DockerHub, quay.io etc.")
		memcachedHostname    = fs.String("memcached-hostname", "", "Hostname for memcached service to use when caching chunks. If empty, no memcached will be used.")
		memcachedTimeout     = fs.Duration("memcached-timeout", 100*time.Millisecond, "Maximum time to wait before giving up on memcached requests.")
		memcachedService     = fs.String("memcached-service", "memcached", "SRV service used to discover memcache servers.")
		registryCacheExpiry  = fs.Duration("registry-cache-expiry", 20*time.Minute, "Duration to keep cached registry tag info. Must be < 1 month.")
		registryPollInterval = fs.Duration("registry-poll-interval", 5*time.Minute, "period at which to poll registry for new images")
		registryRPS          = fs.Int("registry-rps", 200, "maximum registry requests per second per host")
		registryBurst        = fs.Int("registry-burst", 10, "maximum registry request burst per host")
		registryWait         = fs.Duration("registry-wait", time.Second, "maximum wait time when rate limiting")
		// k8s-secret backed ssh keyring configuration
		k8sSecretName            = fs.String("k8s-secret-name", "flux-git-deploy", "Name of the k8s secret used to store the private SSH key")
		k8sSecretVolumeMountPath = fs.String("k8s-secret-volume-mount-path", "/etc/fluxd/ssh", "Mount location of the k8s secret storing the private SSH key")
		k8sSecretDataKey         = fs.String("k8s-secret-data-key", "identity", "Data key holding the private SSH key within the k8s secret")
		// SSH key generation
		sshKeyBits = optionalVar(fs, &ssh.KeyBitsValue{}, "ssh-keygen-bits", "-b argument to ssh-keygen (default unspecified)")
		sshKeyType = optionalVar(fs, &ssh.KeyTypeValue{}, "ssh-keygen-type", "-t argument to ssh-keygen (default unspecified)")

		upstreamURL = fs.String("connect", "", "Connect to an upstream service e.g., Weave Cloud, at this base address")
		token       = fs.String("token", "", "Authentication token for upstream service")
	)
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
		logger = log.NewContext(logger).With("ts", log.DefaultTimestampUTC)
		logger = log.NewContext(logger).With("caller", log.DefaultCaller)
	}

	// Platform component.
	var clusterVersion string
	var sshKeyRing ssh.KeyRing
	var k8s cluster.Cluster
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

		logger := log.NewContext(logger).With("component", "platform")
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
		cluster, err := kubernetes.NewCluster(clientset, kubectlApplier, sshKeyRing, logger)
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
		}

		if err := cluster.Ping(); err != nil {
			logger.Log("ping", err)
		} else {
			logger.Log("ping", true)
		}

		k8s = cluster
		// There is only one way we currently interpret a repo of
		// files as manifests, and that's as Kubernetes yamels.
		k8sManifests = &kubernetes.Manifests{}
	}

	var cache registry.Registry
	{
		var memcacheClient registry.MemcacheClient
		if *memcachedHostname != "" {
			memcacheClient = registry.NewMemcacheClient(registry.MemcacheConfig{
				Host:           *memcachedHostname,
				Service:        *memcachedService,
				Timeout:        *memcachedTimeout,
				UpdateInterval: 1 * time.Minute,
				Logger:         log.NewContext(logger).With("component", "memcached"),
			})
			memcacheClient = registry.InstrumentMemcacheClient(memcacheClient)
			defer memcacheClient.Stop()
		}

		creds, err := registry.CredentialsFromFile(*dockerCredFile)
		if err != nil {
			logger.Log("err", err)
		}
		registryLogger := log.NewContext(logger).With("component", "cache")
		cache = registry.NewRegistry(
			registry.NewCacheClientFactory(creds, registryLogger, memcacheClient, *registryCacheExpiry),
			registryLogger,
		)
		cache = registry.NewInstrumentedRegistry(cache)
	}

	var reg registry.Registry
	{
		creds, err := registry.CredentialsFromFile(*dockerCredFile)
		if err != nil {
			logger.Log("err", err)
		}
		registryLogger := log.NewContext(logger).With("component", "registry")
		reg = registry.NewRegistry(
			registry.NewRemoteClientFactory(creds, registryLogger, registry.RateLimiterConfig{
				RPS:   *registryRPS,
				Burst: *registryBurst,
				Wait:  *registryWait,
			}),
			registryLogger,
		)
		reg = registry.NewInstrumentedRegistry(reg)
	}

	gitRemoteConfig := flux.GitRemoteConfig{
		URL:    *gitURL,
		Branch: *gitBranch,
		Path:   *gitPath,
	}

	// Indirect reference to a daemon, initially of the NotReady variety
	notReadyDaemon := daemon.NewNotReadyDaemon(
		version, k8s, gitRemoteConfig, errors.New("waiting to clone repo"))

	daemonRef := daemon.NewRef(notReadyDaemon)

	var eventWriter history.EventWriter
	{
		// Connect to fluxsvc if given an upstream address
		if *upstreamURL != "" {
			upstreamLogger := log.NewContext(logger).With("component", "upstream")
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
	updateCheckLogger := log.NewContext(logger).With("component", "checkpoint")

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
		}

		for checkout == nil {
			working, err := repo.Clone(gitConfig)
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
					"notes-ref", *gitNotesRef)
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
		V:                    version,
		Cluster:              k8s,
		Manifests:            k8sManifests,
		Registry:             cache,
		Repo:           repo,Checkout:             checkout,
		Jobs:                 jobs,
		JobStatusCache:       &job.StatusCache{Size: 100},

		EventWriter:          eventWriter,
		Logger:               log.NewContext(logger).With("component", "daemon"),LoopVars: &daemon.LoopVars{
			GitPollInterval:      *gitPollInterval,
			RegistryPollInterval: *registryPollInterval,
		},
	}

	shutdownWg.Add(1)
	go daemon.GitPollLoop(shutdown, shutdownWg, log.NewContext(logger).With("component", "sync-loop"))

	// Update daemonRef so that upstream and handlers point to fully working daemon
	daemonRef.UpdatePlatform(daemon)

	// Go!
	logger.Log("exiting", <-errc)
	close(shutdown)
	shutdownWg.Wait()
}

// --- checkpoint: please see https://github.com/weaveworks/go-checkpoint

const (
	versionCheckPeriod = 6 * time.Hour
)

func checkForUpdates(clusterString string, gitString string, logger log.Logger) *checkpoint.Checker {
	handleResponse := func(r *checkpoint.CheckResponse, err error) {
		if err != nil {
			logger.Log("err", err)
			return
		}
		if r.Outdated {
			logger.Log("msg", "update available", "version", r.CurrentVersion, "URL", r.CurrentDownloadURL)
			return
		}
		logger.Log("msg", "up to date", "version", r.CurrentVersion)
	}

	flags := map[string]string{
		"kernel-version":  getKernelVersion(),
		"cluster-version": clusterString,
		"git-configured":  gitString,
	}
	params := checkpoint.CheckParams{
		Product:       "weave-flux",
		Version:       version,
		SignatureFile: "",
		Flags:         flags,
	}

	return checkpoint.CheckInterval(&params, versionCheckPeriod, handleResponse)
}
