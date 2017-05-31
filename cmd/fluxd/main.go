package main

import (
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
	k8sclient "k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/rest"

	//	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/cluster/kubernetes"
	"github.com/weaveworks/flux/daemon"
	"github.com/weaveworks/flux/git"
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
		gitURL      = fs.String("git-url", "", "URL of git repo with Kubernetes manifests; e.g., git@github.com:weaveworks/flux-example")
		gitBranch   = fs.String("git-branch", "master", "branch of git repo to use for Kubernetes manifests")
		gitPath     = fs.String("git-path", "", "path within git repo to locate Kubernetes manifests")
		gitUser     = fs.String("git-user", "Weave Flux", "username to use as git committer")
		gitEmail    = fs.String("git-email", "support@weave.works", "email to use as git committer")
		gitSyncTag  = fs.String("git-sync-tag", "flux-sync", "tag to use to mark sync progress for this cluster")
		gitNotesRef = fs.String("git-notes-ref", "flux", "ref to use for keeping commit annotations in git notes")
		// registry
		dockerCredFile      = fs.String("docker-config", "~/.docker/config.json", "Path to config file with credentials for DockerHub, quay.io etc.")
		memcachedHostname   = fs.String("memcached-hostname", "", "Hostname for memcached service to use when caching chunks. If empty, no memcached will be used.")
		memcachedTimeout    = fs.Duration("memcached-timeout", 100*time.Millisecond, "Maximum time to wait before giving up on memcached requests.")
		memcachedService    = fs.String("memcached-service", "memcached", "SRV service used to discover memcache servers.")
		registryCacheExpiry = fs.Duration("registry-cache-expiry", 20*time.Minute, "Duration to keep cached registry tag info. Must be < 1 month.")
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

		namespace, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
		}

		sshKeyRing, err = kubernetes.NewSSHKeyRing(kubernetes.SSHKeyRingConfig{
			SecretAPI:             clientset.Core().Secrets(string(namespace)),
			SecretName:            "flux-git-deploy",
			SecretVolumeMountPath: "/etc/fluxd/ssh",
			SecretDataKey:         "identity",
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
		logger.Log("host", restClientConfig.Host)

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

	var reg registry.Registry
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
		registryLogger := log.NewContext(logger).With("component", "registry")
		reg = registry.NewRegistry(
			registry.NewRemoteClientFactory(creds, registryLogger, memcacheClient, *registryCacheExpiry),
			registryLogger,
		)
		reg = registry.NewInstrumentedRegistry(reg)
	}

	var checkout *git.Checkout
	{
		repo := git.Repo{
			URL:     *gitURL,
			Path:    *gitPath,
			Branch:  *gitBranch,
			KeyRing: sshKeyRing,
		}
		gitConfig := git.Config{
			SyncTag:   *gitSyncTag,
			NotesRef:  *gitNotesRef,
			UserName:  *gitUser,
			UserEmail: *gitEmail,
		}

		working, err := repo.Clone(gitConfig)
		if err != nil {
			logger.Log("component", "git", "err", err.Error())
			os.Exit(1)
		}

		logger.Log("working-dir", working.Dir,
			"user", *gitUser,
			"email", *gitEmail,
			"sync-tag", *gitSyncTag,
			"notes-ref", *gitNotesRef)
		checkout = working
	}

	shutdown := make(chan struct{})

	queueWg := &sync.WaitGroup{}
	var jobs *job.Queue
	{
		jobs = job.NewQueue()
		queueWg.Add(1)
		go jobs.Loop(shutdown, queueWg)
	}

	daemon := &daemon.Daemon{
		V:              version,
		Cluster:        k8s,
		Manifests:      k8sManifests,
		Registry:       reg,
		Checkout:       checkout,
		Jobs:           jobs,
		JobStatusCache: &job.StatusCache{Size: 100},
	}

	// Connect to fluxsvc if given an upstream address
	if *upstreamURL != "" {
		upstreamLogger := log.NewContext(logger).With("component", "upstream")
		upstreamLogger.Log("URL", *upstreamURL)
		upstream, err := daemonhttp.NewUpstream(
			&http.Client{Timeout: 10 * time.Second},
			fmt.Sprintf("fluxd/%v", version),
			flux.Token(*token),
			transport.NewServiceRouter(), // TODO should be NewUpstreamRouter, since it only needs the registration endpoint
			*upstreamURL,
			&remote.ErrorLoggingPlatform{daemon, upstreamLogger},
			upstreamLogger,
		)
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
		}
		daemon.EventWriter = upstream
		defer upstream.Close()
	} else {
		logger.Log("upstream", "no upstream URL given")
	}

	daemonWg := &sync.WaitGroup{}
	daemonWg.Add(1)
	go daemon.Loop(shutdown, daemonWg, log.NewContext(logger).With("component", "sync-loop"))

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
		handler := daemonhttp.NewHandler(daemon, daemonhttp.NewRouter())
		mux.Handle("/api/flux/", http.StripPrefix("/api/flux", handler))
		logger.Log("addr", *listenAddr)
		errc <- http.ListenAndServe(*listenAddr, mux)
	}()

	// Go!
	logger.Log("exiting", <-errc)
	close(shutdown)
	daemonWg.Wait()
	queueWg.Wait()
}
