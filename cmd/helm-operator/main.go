package main

import (
	"context"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/pflag"

	"fmt"
	"os"
	"os/signal"

	"github.com/go-kit/kit/log"

	clientset "github.com/weaveworks/flux/integrations/client/clientset/versioned"
	ifinformers "github.com/weaveworks/flux/integrations/client/informers/externalversions"
	fluxhelm "github.com/weaveworks/flux/integrations/helm"
	"github.com/weaveworks/flux/integrations/helm/git"
	"github.com/weaveworks/flux/integrations/helm/operator"
	"github.com/weaveworks/flux/integrations/helm/release"
	"github.com/weaveworks/flux/ssh"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	gitssh "gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
)

var (
	fs      *pflag.FlagSet
	err     error
	logger  log.Logger
	kubectl string

	kubeconfig *string
	master     *string

	tillerIP        *string
	tillerPort      *string
	tillerNamespace *string

	crdPollInterval     *time.Duration
	eventHandlerWorkers *uint

	customKubectl *string
	gitURL        *string
	gitBranch     *string
	gitConfigPath *string
	gitChartsPath *string

	k8sSecretName            *string
	k8sSecretVolumeMountPath *string
	k8sSecretDataKey         *string
	sshKeyBits               ssh.OptionalValue
	sshKeyType               ssh.OptionalValue

	upstreamURL      *string
	token            *string
	queueWorkerCount *int

	name       *string
	listenAddr *string
	gcInterval *time.Duration

	ErrOperatorFailure = "Operator failure: %q"
)

const (
	defaultGitConfigPath = "releaseconfig"
	defaultGitChartsPath = "charts"
)

func init() {
	// Flags processing
	fs = pflag.NewFlagSet("default", pflag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "DESCRIPTION\n")
		fmt.Fprintf(os.Stderr, "  helm-operator is a Kubernetes operator for Helm integration into flux.\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "FLAGS\n")
		fs.PrintDefaults()
	}

	kubeconfig = fs.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	master = fs.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")

	tillerIP = fs.String("tiller-ip", "", "Tiller IP address. Only required if out-of-cluster.")
	tillerPort = fs.String("tiller-port", "", "Tiller port. Only required if out-of-cluster.")
	tillerNamespace = fs.String("tiller-namespace", "kube-system", "Tiller namespace. If not provided, the default is kube-system.")

	crdPollInterval = fs.Duration("crd-poll-interval", 5*time.Minute, "Period at which to check for custom resources")
	eventHandlerWorkers = fs.Uint("event-handler-workers", 2, "Number of workers processing events for Flux-Helm custom resources")

	customKubectl = fs.String("kubernetes-kubectl", "", "Optional, explicit path to kubectl tool")
	gitURL = fs.String("git-url", "", "URL of git repo with Kubernetes manifests; e.g., git@github.com:weaveworks/flux-example")
	gitBranch = fs.String("git-branch", "master", "branch of git repo to use for Kubernetes manifests")
	gitChartsPath = fs.String("git-charts-path", defaultGitChartsPath, "path within git repo to locate Helm Charts (relative path)")

	// k8s-secret backed ssh keyring configuration
	k8sSecretName = fs.String("k8s-secret-name", "flux-git-deploy", "Name of the k8s secret used to store the private SSH key")
	k8sSecretVolumeMountPath = fs.String("k8s-secret-volume-mount-path", "/etc/fluxd/ssh", "Mount location of the k8s secret storing the private SSH key")
	k8sSecretDataKey = fs.String("k8s-secret-data-key", "identity", "Data key holding the private SSH key within the k8s secret")
	// SSH key generation
	sshKeyBits = optionalVar(fs, &ssh.KeyBitsValue{}, "ssh-keygen-bitsintegrations/", "-b argument to ssh-keygen (default unspecified)")
	sshKeyType = optionalVar(fs, &ssh.KeyTypeValue{}, "ssh-keygen-type", "-t argument to ssh-keygen (default unspecified)")

	upstreamURL = fs.String("connect", "", "Connect to an upstream service e.g., Weave Cloud, at this base address")
	token = fs.String("token", "", "Authentication token for upstream service")

	queueWorkerCount = fs.Int("queue-worker-count", 2, "Number of workers to process queue with Chart release jobs. Two by default")
}

func main() {

	fs.Parse(os.Args)

	// LOGGING ------------------------------------------------------------------------------
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.With(logger, "ts", log.DefaultTimestampUTC)
		logger = log.With(logger, "caller", log.DefaultCaller)
	}
	// ----------------------------------------------------------------------

	// SHUTDOWN  ----------------------------------------------------------------------------
	errc := make(chan error)

	// Shutdown trigger for goroutines
	shutdown := make(chan struct{})
	shutdownWg := &sync.WaitGroup{}

	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errc <- fmt.Errorf("%s", <-c)
	}()

	defer func() {
		// wait until stopping
		logger.Log("exiting...", <-errc)
		close(shutdown)
		shutdownWg.Wait()
	}()
	// ----------------------------------------------------------------------

	mainLogger := log.With(logger, "component", "helm-operator")
	mainLogger.Log("info", "!!! I am functional! !!!")

	// GIT REPO CONFIG ----------------------------------------------------------------------
	mainLogger.Log("info", "\t*** Setting up git repo configs")
	gitRemoteConfigCh, err := git.NewGitRemoteConfig(*gitURL, *gitBranch, *gitChartsPath)
	if err != nil {
		mainLogger.Log("err", err)
		os.Exit(1)
	}
	fmt.Printf("%#v", gitRemoteConfigCh)
	mainLogger.Log("info", "\t*** Finished setting up git repo configs")

	// CLUSTER ACCESS -----------------------------------------------------------------------
	cfg, err := clientcmd.BuildConfigFromFlags(*master, *kubeconfig)
	if err != nil {
		mainLogger.Log("error", fmt.Sprintf("Error building kubeconfig: %v", err))
		os.Exit(1)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		mainLogger.Log("error", fmt.Sprintf("Error building kubernetes clientset: %v", err))
		os.Exit(1)
	}

	// CUSTOM RESOURCES ----------------------------------------------------------------------
	ifClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		mainLogger.Log("error", fmt.Sprintf("Error building integrations clientset: %v", err))
		//errc <- fmt.Errorf("Error building integrations clientset: %v", err)
		os.Exit(1)
	}

	// HELM ---------------------------------------------------------------------------------
	helmClient, err := fluxhelm.NewClient(kubeClient, fluxhelm.TillerOptions{IP: *tillerIP, Port: *tillerPort, Namespace: *tillerNamespace})
	if err != nil {
		mainLogger.Log("error", fmt.Sprintf("Error creating helm client: %v", err))
		errc <- fmt.Errorf("Error creating helm client: %v", err)
	}
	mainLogger.Log("info", "Set up helmClient")

	//---------------------------------------------------------------------------------------

	// GIT REPO CLONING ---------------------------------------------------------------------
	mainLogger.Log("info", "\t*** Starting to clone repos")

	var gitAuth *gitssh.PublicKeys
	for {
		gitAuth, err = git.GetRepoAuth(*k8sSecretVolumeMountPath, *k8sSecretDataKey)
		if err != nil {
			mainLogger.Log("error", fmt.Sprintf("Failed to set up git authorization : %#v", err))
			//errc <- fmt.Errorf("Failed to create Checkout [%#v]: %v", gitRemoteConfigFhr, err)
			time.Sleep(20 * time.Second)
			continue
		}
		if err == nil {
			break
		}
	}

	// 		Chart releases sync due to pure Charts changes ------------------------------------
	checkoutCh := git.NewCheckout(log.With(logger, "component", "git"), gitRemoteConfigCh, gitAuth)
	defer checkoutCh.Cleanup()

	// If cloning not immediately possible, we wait until it is -----------------------------
	for {
		mainLogger.Log("info", "Cloning repo ...")
		ctx, cancel := context.WithTimeout(context.Background(), git.DefaultCloneTimeout)
		err = checkoutCh.Clone(ctx, git.ChartsChangesClone)
		cancel()
		if err == nil {
			break
		}
		mainLogger.Log("error", fmt.Sprintf("Failed to clone git repo [%s, %s, %s]: %v", gitRemoteConfigCh.URL, gitRemoteConfigCh.Branch, gitRemoteConfigCh.Path, err))
		time.Sleep(10 * time.Second)
	}
	mainLogger.Log("info", "*** Cloned repos")

	// OPERATOR -----------------------------------------------------------------------------
	ifInformerFactory := ifinformers.NewSharedInformerFactory(ifClient, time.Second*30)
	rel := release.New(log.With(logger, "component", "release"), helmClient, checkoutCh)
	opr := operator.New(log.With(logger, "component", "operator"), kubeClient, ifClient, ifInformerFactory, rel)

	// CUSTOM RESOURCES CACHING SETUP -------------------------------------------------------
	go ifInformerFactory.Start(shutdown)

	if err = opr.Run(*queueWorkerCount, shutdown); err != nil {
		msg := fmt.Sprintf("Failure to run controller: %s", err.Error())
		logger.Log("error", msg)
		errc <- fmt.Errorf(ErrOperatorFailure, err)
	}
	//---------------------------------------------------------------------------------------
}

// Helper functions
func optionalVar(fs *pflag.FlagSet, value ssh.OptionalValue, name, usage string) ssh.OptionalValue {
	fs.Var(value, name, usage)
	return value
}
