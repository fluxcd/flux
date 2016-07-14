package main

import (
	"fmt"
	"os"

	"github.com/go-kit/kit/log"
	"github.com/spf13/pflag"
	"k8s.io/kubernetes/pkg/client/restclient"

	"github.com/weaveworks/fluxy/api"
	"github.com/weaveworks/fluxy/platform/kubernetes"
	"github.com/weaveworks/fluxy/registry"
)

func main() {
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "DESCRIPTION\n")
		fmt.Fprintf(os.Stderr, "  fluxd is a deployment daemon.\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "FLAGS\n")
		pflag.PrintDefaults()
	}
	var (
		listenAddr = pflag.StringP("listen", "l", ":3030", "Listen address for Flux API clients")
		// credsPath = pflag.StringP("credentials", "", "", "Path to a credentials file à la Docker, e.g. mounted as a secret")
		kubernetesEnabled       = pflag.BoolP("kubernetes", "", false, "Enable Kubernetes platform")
		kubernetesHost          = pflag.StringP("kubernetes-host", "", "", "Kubernetes host, e.g. http://10.11.12.13:8080")
		kubernetesUsername      = pflag.StringP("kubernetes-username", "", "", "Kubernetes HTTP basic auth username")
		kubernetesPassword      = pflag.StringP("kubernetes-password", "", "", "Kubernetes HTTP basic auth password")
		kubernetesClientCert    = pflag.StringP("kubernetes-client-certificate", "", "", "Path to Kubernetes client certification file for TLS")
		kubernetesClientKey     = pflag.StringP("kubernetes-client-key", "", "", "Path to Kubernetes client key file for TLS")
		kubernetesCertAuthority = pflag.StringP("kubernetes-certificate-authority", "", "", "Path to Kubernetes cert file for certificate authority")

		credsPath = pflag.String("registry-credentials", "", "Path to image registry credentials file, in the format of ~/.docker/config.json")
	)
	pflag.Parse()

	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.NewContext(logger).With("ts", log.DefaultTimestampUTC)
		logger = log.NewContext(logger).With("caller", log.DefaultCaller)
	}

	var k8s *kubernetes.Cluster
	if *kubernetesEnabled {
		logger := log.NewContext(logger).With("platform", "Kubernetes")
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

	creds := registry.NoCredentials()
	if *credsPath != "" {
		logger.Log("creds-file", *credsPath)
		c, err := registry.CredentialsFromFile(*credsPath)
		if err != nil {
			logger.Log("credentials", err)
			os.Exit(1)
		}
		creds = c
	}

	s := &api.Server{
		Platform: k8s,
		Registry: &registry.Client{
			Credentials: creds,
			Logger:      log.NewContext(logger).With("component", "registry"),
		},
	}
	logger.Log("listening", *listenAddr)
	logger.Log("err", s.ListenAndServe(*listenAddr))
}
