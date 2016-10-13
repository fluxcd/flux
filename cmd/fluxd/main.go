package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-kit/kit/log"
	"github.com/spf13/pflag"
	"k8s.io/kubernetes/pkg/client/restclient"

	"github.com/weaveworks/fluxy"
	transport "github.com/weaveworks/fluxy/http"
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
		fluxsvcAddress    = fs.String("fluxsvc-address", "cloud.weave.works:3031", "Address of the fluxsvc to connect to.")
		token             = fs.String("token", "", "Token to use to authenticate with cloud.weave.works")
		kubernetesKubectl = fs.String("kubernetes-kubectl", "", "Optional, explicit path to kubectl tool")
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
		restClientConfig, err := restclient.InClusterConfig()
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
		}

		// When adding a new platform, don't just bash it in. Create a Platform
		// or Cluster interface in package platform, and have kubernetes.Cluster
		// and your new platform implement that interface.
		logger := log.NewContext(logger).With("component", "platform")
		logger.Log("host", restClientConfig.Host)

		k8s, err = kubernetes.NewCluster(restClientConfig, *kubernetesKubectl, logger)
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
	client, err := transport.NewDaemon(http.DefaultClient, flux.Token(*token), transport.NewRouter(), *fluxsvcAddress, k8s, clientLogger)
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
