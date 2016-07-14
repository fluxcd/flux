package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-kit/kit/log"
	"github.com/spf13/pflag"
	"golang.org/x/net/context"
	"k8s.io/kubernetes/pkg/client/restclient"

	"github.com/weaveworks/fluxy"
	"github.com/weaveworks/fluxy/platform/kubernetes"
	"github.com/weaveworks/fluxy/registry"
)

func main() {
	// Flag domain.
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "DESCRIPTION\n")
		fmt.Fprintf(os.Stderr, "  fluxd is a deployment daemon.\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "FLAGS\n")
		pflag.PrintDefaults()
	}
	var (
		listenAddr              = pflag.StringP("listen", "l", ":3030", "Listen address for Flux API clients")
		registryCredentials     = pflag.StringP("registry-credentials", "", "", "Path to image registry credentials file, in the format of ~/.docker/config.json")
		kubernetesEnabled       = pflag.BoolP("kubernetes", "", false, "Enable Kubernetes platform")
		kubernetesHost          = pflag.StringP("kubernetes-host", "", "", "Kubernetes host, e.g. http://10.11.12.13:8080")
		kubernetesUsername      = pflag.StringP("kubernetes-username", "", "", "Kubernetes HTTP basic auth username")
		kubernetesPassword      = pflag.StringP("kubernetes-password", "", "", "Kubernetes HTTP basic auth password")
		kubernetesClientCert    = pflag.StringP("kubernetes-client-certificate", "", "", "Path to Kubernetes client certification file for TLS")
		kubernetesClientKey     = pflag.StringP("kubernetes-client-key", "", "", "Path to Kubernetes client key file for TLS")
		kubernetesCertAuthority = pflag.StringP("kubernetes-certificate-authority", "", "", "Path to Kubernetes cert file for certificate authority")
	)
	pflag.Parse()

	// Logger domain.
	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.NewContext(logger).With("ts", log.DefaultTimestampUTC)
		logger = log.NewContext(logger).With("caller", log.DefaultCaller)
	}

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
	if *kubernetesEnabled {
		logger := log.NewContext(logger).With("component", "Kubernetes")
		logger.Log("host", kubernetesHost)

		var err error
		k8s, err = kubernetes.NewCluster(&restclient.Config{
			Host:     *kubernetesHost,
			Username: *kubernetesUsername,
			Password: *kubernetesPassword,
			TLSClientConfig: restclient.TLSClientConfig{
				CertFile: *kubernetesClientCert,
				KeyFile:  *kubernetesClientKey,
				CAFile:   *kubernetesCertAuthority,
			},
		}, logger)
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

	// Service (business logic) domain.
	var service flux.Service
	{
		service = flux.NewService(reg, k8s)
		service = flux.LoggingMiddleware(logger)(service)
	}

	// Endpoint domain.
	var endpoints flux.Endpoints
	{
		endpoints = flux.MakeServerEndpoints(service)
	}

	// Mechanical stuff.
	errc := make(chan error)
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errc <- fmt.Errorf("%s", <-c)
	}()

	// Transport domain.
	ctx := context.Background()
	go func() {
		logger := log.NewContext(logger).With("transport", "HTTP")
		logger.Log("addr", *listenAddr)
		h := flux.MakeHTTPHandler(ctx, endpoints, logger)
		errc <- http.ListenAndServe(*listenAddr, h)
	}()

	// Go!
	logger.Log("exit", <-errc)
}
