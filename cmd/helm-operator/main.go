package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/spf13/pflag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/weaveworks/flux/git"
	clientset "github.com/weaveworks/flux/integrations/client/clientset/versioned"
	ifinformers "github.com/weaveworks/flux/integrations/client/informers/externalversions"
	fluxhelm "github.com/weaveworks/flux/integrations/helm"
	helmop "github.com/weaveworks/flux/integrations/helm"
	"github.com/weaveworks/flux/integrations/helm/chartsync"
	"github.com/weaveworks/flux/integrations/helm/operator"
	"github.com/weaveworks/flux/integrations/helm/release"
	"github.com/weaveworks/flux/integrations/helm/status"
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

	tillerTLSVerify *bool
	tillerTLSEnable *bool
	tillerTLSKey    *string
	tillerTLSCert   *string
	tillerTLSCACert *string

	chartsSyncInterval *time.Duration
	chartsSyncTimeout  *time.Duration

	gitURL          *string
	gitBranch       *string
	gitChartsPath   *string
	gitPollInterval *time.Duration

	queueWorkerCount *int

	name       *string
	listenAddr *string
	gcInterval *time.Duration
)

const (
	defaultGitChartsPath = "charts"

	ErrOperatorFailure = "Operator failure: %q"
)

func init() {
	// Flags processing
	fs = pflag.NewFlagSet("default", pflag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "DESCRIPTION\n")
		fmt.Fprintf(os.Stderr, "  helm-operator releases Helm charts from git.\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "FLAGS\n")
		fs.PrintDefaults()
	}

	kubeconfig = fs.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	master = fs.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")

	tillerIP = fs.String("tiller-ip", "", "Tiller IP address. Only required if out-of-cluster.")
	tillerPort = fs.String("tiller-port", "", "Tiller port.")
	tillerNamespace = fs.String("tiller-namespace", "kube-system", "Tiller namespace. If not provided, the default is kube-system.")

	tillerTLSVerify = fs.Bool("tiller-tls-verify", false, "Verify TLS certificate from Tiller. Will enable TLS communication when provided.")
	tillerTLSEnable = fs.Bool("tiller-tls-enable", false, "Enable TLS communication with Tiller. If provided, requires TLSKey and TLSCert to be provided as well.")
	tillerTLSKey = fs.String("tiller-tls-key-path", "/etc/fluxd/helm/tls.key", "Path to private key file used to communicate with the Tiller server.")
	tillerTLSCert = fs.String("tiller-tls-cert-path", "/etc/fluxd/helm/tls.crt", "Path to certificate file used to communicate with the Tiller server.")
	tillerTLSCACert = fs.String("tiller-tls-ca-cert-path", "", "Path to CA certificate file used to validate the Tiller server. Required if tiller-tls-verify is enabled.")

	chartsSyncInterval = fs.Duration("charts-sync-interval", 3*time.Minute, "Interval at which to check for changed charts")
	chartsSyncTimeout = fs.Duration("charts-sync-timeout", 1*time.Minute, "Timeout when checking for changed charts")

	gitURL = fs.String("git-url", "", "URL of git repo with Helm Charts; e.g., git@github.com:weaveworks/flux-example")
	gitBranch = fs.String("git-branch", "master", "branch of git repo")
	gitChartsPath = fs.String("git-charts-path", defaultGitChartsPath, "path within git repo to locate Helm Charts (relative path)")
	gitPollInterval = fs.Duration("git-poll-interval", 5*time.Minute, "period on which to poll for changes to the git repo")

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
		logger.Log("exiting...", <-errc)
		close(shutdown)
		shutdownWg.Wait()
	}()

	mainLogger := log.With(logger, "component", "helm-operator")

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

	// CUSTOM RESOURCES CLIENT --------------------------------------------------------------
	ifClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		mainLogger.Log("error", fmt.Sprintf("Error building integrations clientset: %v", err))
		//errc <- fmt.Errorf("Error building integrations clientset: %v", err)
		os.Exit(1)
	}

	// HELM ---------------------------------------------------------------------------------
	helmClient := fluxhelm.ClientSetup(log.With(logger, "component", "helm"), kubeClient, fluxhelm.TillerOptions{
		IP:        *tillerIP,
		Port:      *tillerPort,
		Namespace: *tillerNamespace,
		TLSVerify: *tillerTLSVerify,
		TLSEnable: *tillerTLSEnable,
		TLSKey:    *tillerTLSKey,
		TLSCert:   *tillerTLSCert,
		TLSCACert: *tillerTLSCACert,
	})

	// The status updater, to keep track the release status for each
	// FluxHelmRelease. It runs as a separate loop for now.
	statusUpdater := status.New(ifClient, kubeClient, helmClient)
	go statusUpdater.Loop(shutdown, log.With(logger, "component", "annotator"))

	gitRemote := git.Remote{URL: *gitURL}
	repo := git.NewRepo(gitRemote, git.PollInterval(*gitPollInterval), git.ReadOnly)

	// 		Chart releases sync due to Custom Resources changes -------------------------------
	{
		mainLogger.Log("info", "Attempting to clone repo ...", "url", gitRemote.URL)
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		err := repo.Ready(ctx)
		cancel()
		if err != nil {
			mainLogger.Log("error", err)
			os.Exit(2)
		}
		mainLogger.Log("info", "Repo cloned", "url", gitRemote.URL)

		// Start the repo fetching from upstream
		shutdownWg.Add(1)
		go func() {
			errc <- repo.Start(shutdown, shutdownWg)
		}()
	}

	releaseConfig := release.Config{
		ChartsPath: *gitChartsPath,
	}
	repoConfig := helmop.RepoConfig{
		Repo:       repo,
		Branch:     *gitBranch,
		ChartsPath: *gitChartsPath,
	}

	// release instance is needed during the sync of Charts changes and during the sync of FluxHelmRelease changes
	rel := release.New(log.With(logger, "component", "release"), helmClient, releaseConfig)
	// CHARTS CHANGES SYNC ------------------------------------------------------------------
	chartSync := chartsync.New(log.With(logger, "component", "chartsync"),
		chartsync.Polling{Interval: *chartsSyncInterval, Timeout: *chartsSyncTimeout},
		chartsync.Clients{KubeClient: *kubeClient, IfClient: *ifClient},
		rel, repoConfig)
	chartSync.Run(shutdown, errc, shutdownWg)

	// OPERATOR - CUSTOM RESOURCE CHANGE SYNC -----------------------------------------------
	// CUSTOM RESOURCES CACHING SETUP -------------------------------------------------------
	//				SharedInformerFactory sets up informer, that maps resource type to a cache shared informer.
	//				operator attaches event handler to the informer and syncs the informer cache
	ifInformerFactory := ifinformers.NewSharedInformerFactory(ifClient, 30*time.Second)
	// Reference to shared index informers for the FluxHelmRelease
	fhrInformer := ifInformerFactory.Helm().V1alpha2().FluxHelmReleases()

	opr := operator.New(log.With(logger, "component", "operator"), kubeClient, fhrInformer, rel, repoConfig)
	// Starts handling k8s events related to the given resource kind
	go ifInformerFactory.Start(shutdown)

	if err = opr.Run(*queueWorkerCount, shutdown, shutdownWg); err != nil {
		msg := fmt.Sprintf("Failure to run controller: %s", err.Error())
		logger.Log("error", msg)
		errc <- fmt.Errorf(ErrOperatorFailure, err)
	}
}
