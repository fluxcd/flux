package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-kit/kit/log"
	"github.com/spf13/pflag"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"

	"github.com/weaveworks/fluxy/platform/client"
	"github.com/weaveworks/fluxy/platform/kubernetes"
)

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
		fluxsvcAddress            = fs.String("fluxsvc-address", "cloud.weave.works", "Address of the fluxsvc to connect to.")
		token                     = fs.String("token", "", "Token to use to authenticate with cloud.weave.works")
		insecure                  = fs.Bool("insecure", false, "Explicitly allow \"insecure\" SSL connections")
		kubernetesMinikube        = fs.Bool("kubernetes-minikube", false, "Parse Kubernetes access information from standard minikube files")
		kubernetesKubectl         = fs.String("kubernetes-kubectl", "", "Optional, explicit path to kubectl tool")
		kubernetesHost            = fs.String("kubernetes-host", "", "Kubernetes host, e.g. http://10.11.12.13:8080")
		kubernetesUsername        = fs.String("kubernetes-username", "", "Kubernetes HTTP basic auth username")
		kubernetesPassword        = fs.String("kubernetes-password", "", "Kubernetes HTTP basic auth password")
		kubernetesClientCert      = fs.String("kubernetes-client-certificate", "", "Path to Kubernetes client certification file for TLS")
		kubernetesClientKey       = fs.String("kubernetes-client-key", "", "Path to Kubernetes client key file for TLS")
		kubernetesCertAuthority   = fs.String("kubernetes-certificate-authority", "", "Path to Kubernetes cert file for certificate authority")
		kubernetesBearerTokenFile = fs.String("kubernetes-bearer-token-file", "", "Path to file containing Kubernetes Bearer Token file")
	)
	fs.Parse(os.Args)

	// Logger component.
	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.NewContext(logger).With("ts", log.DefaultTimestampUTC)
		logger = log.NewContext(logger).With("caller", log.DefaultCaller)
	}

	// Platform component.
	var k8s *kubernetes.Cluster
	{
		var restClientConfig *restclient.Config

		if *kubernetesMinikube {
			loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
			configOverrides := &clientcmd.ConfigOverrides{}

			// TODO: handle the filename for kubeconfig here, as well.
			// Or just use a NewInCluster
			kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
			var err error
			restClientConfig, err = kubeConfig.ClientConfig()
			if err != nil {
				logger.Log("err", err)
				os.Exit(1)
			}
		}

		if restClientConfig == nil {
			var bearerToken string
			if *kubernetesBearerTokenFile != "" {
				buf, err := ioutil.ReadFile(*kubernetesBearerTokenFile)
				if err != nil {
					logger.Log("err", err)
					os.Exit(1)
				}
				bearerToken = string(buf)
			}
			restClientConfig = &restclient.Config{
				Host:        *kubernetesHost,
				Username:    *kubernetesUsername,
				Password:    *kubernetesPassword,
				BearerToken: bearerToken,
				TLSClientConfig: restclient.TLSClientConfig{
					CertFile: *kubernetesClientCert,
					KeyFile:  *kubernetesClientKey,
					CAFile:   *kubernetesCertAuthority,
				},
			}
		}

		// When adding a new platform, don't just bash it in. Create a Platform
		// or Cluster interface in package platform, and have kubernetes.Cluster
		// and your new platform implement that interface.
		logger := log.NewContext(logger).With("component", "platform")
		logger.Log("host", restClientConfig.Host)

		var err error
		k8s, err := kubernetes.NewCluster(restClientConfig, *kubernetesKubectl, logger)
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
		}

		if services, err := k8s.AllServices("", nil); err != nil {
			logger.Log("services", err)
		} else {
			logger.Log("services", len(services))
		}
	}

	// Connect to fluxsvc
	clientLogger := log.NewContext(logger).With("component", "client")
	client, err := client.New(*fluxsvcAddress, *token, *insecure, k8s, clientLogger)
	if err != nil {
		logger.Log("err", err)
		os.Exit(1)
	}
	defer client.Close()

	// Mechanical components.
	errc := make(chan error)
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errc <- fmt.Errorf("%s", <-c)
	}()

	// Go!
	logger.Log("exit", <-errc)
}
