package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"

	"github.com/weaveworks/fluxy/flux/fluxd"
	"github.com/weaveworks/fluxy/flux/platform/kubernetes"
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

	// Logging.
	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.NewContext(logger).With("ts", log.DefaultTimestampUTC)
		logger = log.NewContext(logger).With("caller", log.DefaultCaller)
	}

	// Instrumentation.
	var (
		httpDuration metrics.Histogram
		metrics      fluxd.Metrics
	)
	{
		metrics.ListServicesDuration = prometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
			Namespace: "flux",
			Subsystem: "fluxd",
			Name:      "list_services_duration_seconds",
			Help:      "ListServices method duration in seconds.",
		}, []string{"namespace", "success"})
		metrics.ListImagesDuration = prometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
			Namespace: "flux",
			Subsystem: "fluxd",
			Name:      "list_images_duration_seconds",
			Help:      "ListImages method duration in seconds.",
		}, []string{"service_spec", "success"})
		metrics.ReleaseDuration = prometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
			Namespace: "flux",
			Subsystem: "fluxd",
			Name:      "release_duration_seconds",
			Help:      "Release method duration in seconds.",
		}, []string{"kind", "success"})
		httpDuration = prometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
			Namespace: "flux",
			Subsystem: "fluxd",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
		}, []string{"method", "status_code"})
	}

	// Platform.
	var cluster *kubernetes.Cluster
	{
		var restClientConfig *restclient.Config

		if *kubernetesMinikube {
			loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
			configOverrides := &clientcmd.ConfigOverrides{}

			// TODO: handle the filename for kubeconfig here, as well.
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
		cluster, err = kubernetes.NewCluster(restClientConfig, *kubernetesKubectl, logger)
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
		}
	}

	// Service.
	var service fluxd.Service
	{
		service = fluxd.NewServer(cluster, logger, metrics)
	}

	// Mechanical stuff.
	errc := make(chan error)
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errc <- fmt.Errorf("%s", <-c)
	}()

	// HTTP transport component.
	go func() {
		logger.Log("addr", *listenAddr)
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		mux.Handle("/", fluxd.NewHandler(service, fluxd.NewRouter(), logger, httpDuration))
		errc <- http.ListenAndServe(*listenAddr, mux)
	}()

	// Go!
	logger.Log("exit", <-errc)
}
