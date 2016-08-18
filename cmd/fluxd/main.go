package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"regexp"
	"syscall"

	"github.com/go-kit/kit/log"
	"github.com/spf13/pflag"
	"k8s.io/kubernetes/pkg/client/restclient"

	"github.com/weaveworks/fluxy"
	"github.com/weaveworks/fluxy/automator"
	"github.com/weaveworks/fluxy/git"
	"github.com/weaveworks/fluxy/history"
	"github.com/weaveworks/fluxy/platform/kubernetes"
	"github.com/weaveworks/fluxy/registry"
)

func main() {
	// Flag domain.
	fs := pflag.NewFlagSet("default", pflag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "DESCRIPTION\n")
		fmt.Fprintf(os.Stderr, "  fluxd is a deployment daemon.\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "FLAGS\n")
		fs.PrintDefaults()
	}
	// This mirrors how kubectl extracts information from the environment.
	var (
		listenAddr                = fs.StringP("listen", "l", ":3030", "Listen address for Flux API clients")
		registryCredentials       = fs.String("registry-credentials", "", "Path to image registry credentials file, in the format of ~/.docker/config.json")
		kubernetesMinikube        = fs.Bool("kubernetes-minikube", false, "Parse Kubernetes access information from standard minikube files")
		kubernetesKubectl         = fs.String("kubernetes-kubectl", "", "Optional, explicit path to kubectl tool")
		kubernetesHost            = fs.String("kubernetes-host", "", "Kubernetes host, e.g. http://10.11.12.13:8080")
		kubernetesUsername        = fs.String("kubernetes-username", "", "Kubernetes HTTP basic auth username")
		kubernetesPassword        = fs.String("kubernetes-password", "", "Kubernetes HTTP basic auth password")
		kubernetesClientCert      = fs.String("kubernetes-client-certificate", "", "Path to Kubernetes client certification file for TLS")
		kubernetesClientKey       = fs.String("kubernetes-client-key", "", "Path to Kubernetes client key file for TLS")
		kubernetesCertAuthority   = fs.String("kubernetes-certificate-authority", "", "Path to Kubernetes cert file for certificate authority")
		kubernetesBearerTokenFile = fs.String("kubernetes-bearer-token-file", "", "Path to file containing Kubernetes Bearer Token file")
		databaseDriver            = fs.String("database-driver", "ql-mem", `Database driver name, e.g., "postgres"; the default is an in-memory DB`)
		databaseSource            = fs.String("database-source", "history.db", `Database source name; specific to the database driver (--database-driver) used. The default is an arbitrary, in-memory DB name`)
		repoURL                   = fs.String("repo-url", "", "Config repo URL, e.g. https://github.com/myorg/conf.git (required)")
		repoKey                   = fs.String("repo-key", "", "SSH key file with commit rights to config repo")
		repoPath                  = fs.String("repo-path", "", "Path within config repo to look for resource definition files")
	)
	fs.Parse(os.Args)

	// Logger component.
	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.NewContext(logger).With("ts", log.DefaultTimestampUTC)
		logger = log.NewContext(logger).With("caller", log.DefaultCaller)
	}

	// Registry component.
	var reg *registry.Client
	{
		logger := log.NewContext(logger).With("component", "registry")
		creds := registry.NoCredentials()
		if *registryCredentials != "" {
			logger.Log("credentials", *registryCredentials)
			c, err := registry.CredentialsFromFile(*registryCredentials)
			if err != nil {
				logger.Log("err", err)
				os.Exit(1)
			}
			creds = c
		} else {
			logger.Log("credentials", "none")
		}
		reg = &registry.Client{
			Credentials: creds,
			Logger:      logger,
		}
	}

	// Platform component.
	var k8s *kubernetes.Cluster
	{
		if *kubernetesMinikube {
			*kubernetesHost = parseMinikubeHost()
			*kubernetesClientCert = filepath.Join(homeDir(), ".minikube", "apiserver.crt")
			*kubernetesClientKey = filepath.Join(homeDir(), ".minikube", "apiserver.key")
			*kubernetesCertAuthority = filepath.Join(homeDir(), ".minikube", "ca.crt")
		}

		// When adding a new platform, don't just bash it in. Create a Platform
		// or Cluster interface in package platform, and have kubernetes.Cluster
		// and your new platform implement that interface.
		logger := log.NewContext(logger).With("component", "platform")
		logger.Log("host", kubernetesHost)

		var bearerToken string
		if *kubernetesBearerTokenFile != "" {
			buf, err := ioutil.ReadFile(*kubernetesBearerTokenFile)
			if err != nil {
				logger.Log("err", err)
				os.Exit(1)
			}
			bearerToken = string(buf)
		}

		var err error
		k8s, err = kubernetes.NewCluster(&restclient.Config{
			Host:        *kubernetesHost,
			Username:    *kubernetesUsername,
			Password:    *kubernetesPassword,
			BearerToken: bearerToken,
			TLSClientConfig: restclient.TLSClientConfig{
				CertFile: *kubernetesClientCert,
				KeyFile:  *kubernetesClientKey,
				CAFile:   *kubernetesCertAuthority,
			},
		}, *kubernetesKubectl, logger)
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
		}

		if services, err := k8s.Services("default"); err != nil {
			logger.Log("services", err)
		} else {
			logger.Log("services", len(services))
		}
	}

	// History component.
	var his history.DB
	{
		var err error
		his, err = history.NewSQL(*databaseDriver, *databaseSource)
		if err != nil {
			logger.Log("component", "history", "err", err)
			os.Exit(1)
		}
	}

	// Automator component.
	var auto *automator.Automator
	{
		var err error
		auto, err = automator.New(automator.Config{
			Platform: k8s,
			Registry: reg,
			History:  his,
			Repo: git.Repo{
				URL:  *repoURL,
				Key:  *repoKey,
				Path: *repoPath,
			},
		})
		if err == nil {
			logger.Log("automator", "enabled", "repo", *repoURL)
		} else {
			// Service can handle a nil automator pointer.
			logger.Log("automator", "disabled", "reason", err)
		}
	}

	// Server component.
	server := flux.NewServer(k8s, reg, auto, his)

	// Mechanical components.
	errc := make(chan error)
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errc <- fmt.Errorf("%s", <-c)
	}()

	// HTTP transport component.
	go func() {
		logger.Log("addr", *listenAddr)
		h := flux.NewHandler(server, flux.NewRouter())
		errc <- http.ListenAndServe(*listenAddr, h)
	}()

	// Go!
	logger.Log("exit", <-errc)
}

func parseMinikubeHost() string {
	buf, err := ioutil.ReadFile(filepath.Join(homeDir(), ".kube", "config"))
	if err != nil {
		return "minikube-host-unavailable"
	}
	matches := regexp.MustCompile(`server: (https://192\.168\.64\.[0-9]+:[0-9]+)`).FindSubmatch(buf)
	if len(matches) < 2 {
		return "minikube-host-unavailable"
	}
	return string(matches[1])
}

func homeDir() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}
	return u.HomeDir
}
