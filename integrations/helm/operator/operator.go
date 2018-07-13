package operator

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	ifv1 "github.com/weaveworks/flux/apis/helm.integrations.flux.weave.works/v1alpha2"
	ifscheme "github.com/weaveworks/flux/integrations/client/clientset/versioned/scheme"
	fhrv1 "github.com/weaveworks/flux/integrations/client/informers/externalversions/helm.integrations.flux.weave.works/v1alpha2"
	iflister "github.com/weaveworks/flux/integrations/client/listers/helm.integrations.flux.weave.works/v1alpha2"
	helmop "github.com/weaveworks/flux/integrations/helm"
	chartrelease "github.com/weaveworks/flux/integrations/helm/release"
)

const (
	controllerAgentName = "helm-operator"
	CacheSyncTimeout    = 180 * time.Second
)

const (
	// ChartSynced is used as part of the Event 'reason' when the Chart related to the
	// a FluxHelmRelease gets released/updated
	ChartSynced = "ChartSynced"
	// ErrChartSync is used as part of the Event 'reason' when the related Chart related to the
	// a FluxHelmRelease fails to be released/updated
	ErrChartSync = "ErrChartSync"

	// MessageChartSynced - the message used for Events when a resource
	// fails to sync due to failing to release the Chart
	MessageChartSynced = "Chart managed by FluxHelmRelease processed successfully"
	// MessageErrChartSync - the message used for an Event fired when a FluxHelmRelease
	// is synced successfully
	MessageErrChartSync = "Chart %s managed by FluxHelmRelease failed to be processed"
)

// Controller is the operator implementation for FluxHelmRelease resources
type Controller struct {
	logger log.Logger

	fhrLister iflister.FluxHelmReleaseLister
	fhrSynced cache.InformerSynced

	release *chartrelease.Release
	config  helmop.RepoConfig

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	releaseWorkqueue workqueue.RateLimitingInterface

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// New returns a new helm-operator
func New(
	logger log.Logger,
	kubeclientset kubernetes.Interface,
	fhrInformer fhrv1.FluxHelmReleaseInformer,
	release *chartrelease.Release,
	config helmop.RepoConfig) *Controller {

	// Add helm-operator types to the default Kubernetes Scheme so Events can be
	// logged for helm-operator types.
	ifscheme.AddToScheme(scheme.Scheme)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		logger:           logger,
		fhrLister:        fhrInformer.Lister(),
		fhrSynced:        fhrInformer.Informer().HasSynced,
		release:          release,
		releaseWorkqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ChartRelease"),
		recorder:         recorder,
		config:           config,
	}

	controller.logger.Log("info", "Setting up event handlers")

	// ----- EVENT HANDLERS for FluxHelmRelease resources change ---------
	fhrInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(new interface{}) {
			controller.logger.Log("info", "CREATING release")
			controller.logger.Log("info", "Custom Resource driven release install")
			_, ok := checkCustomResourceType(controller.logger, new)
			if ok {
				controller.enqueueJob(new)
			}
		},
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueUpateJob(old, new)
		},
		DeleteFunc: func(old interface{}) {
			fhr, ok := checkCustomResourceType(controller.logger, old)
			if ok {
				controller.deleteRelease(fhr)
			}
		},
	})
	controller.logger.Log("info", "Event handlers set up")

	return controller
}

// Run sets up the event handlers for our Custom Resource, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}, wg *sync.WaitGroup) error {
	defer runtime.HandleCrash()
	defer c.releaseWorkqueue.ShutDown()

	c.logger.Log("info", "Starting operator")
	// Wait for the caches to be synced before starting workers
	c.logger.Log("info", "Waiting for informer caches to sync")

	if ok := cache.WaitForCacheSync(stopCh, c.fhrSynced); !ok {
		return errors.New("failed to wait for caches to sync")
	}
	c.logger.Log("info", "Informer caches synced")

	c.logger.Log("info", "Starting workers")
	for i := 0; i < threadiness; i++ {
		wg.Add(1)
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	for i := 0; i < threadiness; i++ {
		wg.Done()
	}
	c.logger.Log("info", "Stopping workers")

	return nil
}

// runWorker is a long-running function calling the
// processNextWorkItem function to read and process a message
// on a workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	c.logger.Log("debug", "Processing next work queue job ...")

	obj, shutdown := c.releaseWorkqueue.Get()
	c.logger.Log("debug", fmt.Sprintf("PROCESSING item [%#v]", obj))

	if shutdown {
		return false
	}

	// wrapping block in a func to defer c.workqueue.Done
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We must call Forget if we do not want
		// this work item being re-queued. If a transient error
		// occurs, we do not call Forget. Instead the item is put back
		// on the workqueue and attempted again after a back-off
		// period.
		defer c.releaseWorkqueue.Done(obj)

		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form "namespace/fhr(custom resource) name". We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date than when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget not to get into a loop of attempting to
			// process a work item that is invalid.
			c.releaseWorkqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("Expected string in workqueue but got %#v", obj))

			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// FluxHelmRelease resource to sync the corresponding Chart release.
		// If the sync failed, then we return while the item will get requeued
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// If no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.releaseWorkqueue.Forget(obj)

		c.logger.Log("info", fmt.Sprintf("Successfully synced '%s'", key))

		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}
	return true
}

// syncHandler acts according to the action
// 		Deletes/creates or updates a Chart release
func (c *Controller) syncHandler(key string) error {
	c.logger.Log("debug", fmt.Sprintf("Starting to sync cache key %s", key))

	// Retrieve namespace and Custom Resource name from the key
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		c.logger.Log("info", fmt.Sprintf("Invalid cache key: %v", err))
		runtime.HandleError(fmt.Errorf("Invalid cache key: %s", key))
		return nil
	}

	// Custom Resource fhr contains all information we need to know about the Chart release
	fhr, err := c.fhrLister.FluxHelmReleases(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			c.logger.Log("info", fmt.Sprintf("FluxHelmRelease '%s' referred to in work queue no longer exists", key))
			runtime.HandleError(fmt.Errorf("FluxHelmRelease '%s' referred to in work queue no longer exists", key))
			return nil
		}
		c.logger.Log("error", err.Error())
		return err
	}

	var syncType chartrelease.Action

	releaseName := chartrelease.GetReleaseName(*fhr)
	ok, err := c.release.Exists(releaseName)
	if ok {
		if err != nil {
			c.logger.Log("error", fmt.Sprintf("Failure to do Chart release [%s]: %#v", releaseName, err))
			return err
		}
		syncType = chartrelease.UpgradeAction
	}
	if !ok {
		syncType = chartrelease.InstallAction
	}

	// Chart installation of the appropriate type
	ctx, cancel := context.WithTimeout(context.Background(), helmop.GitOperationTimeout)
	clone, err := c.config.Repo.Export(ctx, c.config.Branch)
	cancel()
	if err != nil {
		return fmt.Errorf("Failure to clone repo: %s", err.Error())
	}
	defer clone.Clean()

	opts := chartrelease.InstallOptions{DryRun: false}
	_, err = c.release.Install(clone.Dir(), releaseName, *fhr, syncType, opts)
	if err != nil {
		return err
	}

	c.recorder.Event(fhr, corev1.EventTypeNormal, ChartSynced, MessageChartSynced)
	return nil
}

func checkCustomResourceType(logger log.Logger, obj interface{}) (ifv1.FluxHelmRelease, bool) {
	var fhr *ifv1.FluxHelmRelease
	var ok bool
	if fhr, ok = obj.(*ifv1.FluxHelmRelease); !ok {
		logger.Log("error", fmt.Sprintf("FluxHelmRelease Event Watch received an invalid object: %#v", obj))
		return ifv1.FluxHelmRelease{}, false
	}
	return *fhr, true
}

func getCacheKey(obj interface{}) (string, error) {
	var key string
	var err error

	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return "", err
	}
	return key, nil
}

// enqueueJob takes a FluxHelmRelease resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should not be
// passed resources of any type other than FluxHelmRelease.
func (c *Controller) enqueueJob(obj interface{}) {
	var key string
	var err error
	if key, err = getCacheKey(obj); err != nil {
		return
	}
	c.releaseWorkqueue.AddRateLimited(key)
}

// enqueueUpdateJob decides if there is a genuine resource update
func (c *Controller) enqueueUpateJob(old, new interface{}) {
	oldFhr, ok := checkCustomResourceType(c.logger, old)
	if !ok {
		return
	}
	newFhr, ok := checkCustomResourceType(c.logger, new)
	if !ok {
		return
	}

	if needsUpdate(oldFhr, newFhr) {
		c.logger.Log("info", "UPGRADING release")
		c.logger.Log("info", "Custom Resource driven release upgrade")
		c.enqueueJob(new)
	}
}

func (c *Controller) deleteRelease(fhr ifv1.FluxHelmRelease) {
	c.logger.Log("info", "DELETING release")
	c.logger.Log("info", "Custom Resource driven release deletion")
	name := chartrelease.GetReleaseName(fhr)
	err := c.release.Delete(name)
	if err != nil {
		c.logger.Log("error", fmt.Sprintf("Chart release [%s] not deleted: %#v", name, err))
	}
	return
}

// needsUpdate compares two FluxHelmRelease and determines if any changes occurred
func needsUpdate(old, new ifv1.FluxHelmRelease) bool {
	oldValues, err := old.Spec.Values.YAML()
	if err != nil {
		return false
	}

	newValues, err := new.Spec.Values.YAML()
	if err != nil {
		return false
	}

	if oldValues != newValues {
		return true
	}

	if old.Spec.ReleaseName != new.Spec.ReleaseName {
		return true
	}

	if old.Spec.ChartGitPath != new.Spec.ChartGitPath {
		return true
	}

	return false
}
