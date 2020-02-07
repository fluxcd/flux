package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/argoproj/argo-cd/engine/pkg/engine"
	"github.com/argoproj/argo-cd/engine/pkg/utils/kube"
	"github.com/argoproj/argo-cd/engine/pkg/utils/kube/cache"
	hrclient "github.com/fluxcd/helm-operator/pkg/client/clientset/versioned"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"github.com/ryanuber/go-glob"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	fhrclient "github.com/fluxcd/flux/integrations/client/clientset/versioned"
	"github.com/fluxcd/flux/pkg/cluster"
	kresource "github.com/fluxcd/flux/pkg/cluster/kubernetes/resource"
	"github.com/fluxcd/flux/pkg/resource"
	"github.com/fluxcd/flux/pkg/ssh"
)

type coreClient k8sclient.Interface
type fluxHelmClient fhrclient.Interface
type helmOperatorClient hrclient.Interface
type discoveryClient discovery.DiscoveryInterface

type ExtendedClient struct {
	coreClient
	fluxHelmClient
	helmOperatorClient
}

func MakeClusterClientset(core coreClient, fluxhelm fluxHelmClient, helmop helmOperatorClient) ExtendedClient {

	return ExtendedClient{
		coreClient:         core,
		fluxHelmClient:     fluxhelm,
		helmOperatorClient: helmop,
	}
}

// --- add-ons

// Kubernetes has a mechanism of "Add-ons", whereby manifest files
// left in a particular directory on the Kubernetes master will be
// applied. We can recognise these, because they:
//  1. Must be in the namespace `kube-system`; and,
//  2. Must have one of the labels below set, else the addon manager will ignore them.
//
// We want to ignore add-ons, since they are managed by the add-on
// manager, and attempts to control them via other means will fail.

// k8sObject represents an value from which you can obtain typical
// Kubernetes metadata. These methods are implemented by the
// Kubernetes API resource types.
type k8sObject interface {
	GetName() string
	GetNamespace() string
	GetLabels() map[string]string
	GetAnnotations() map[string]string
}

func isAddon(obj k8sObject) bool {
	if obj.GetNamespace() != "kube-system" {
		return false
	}
	labels := obj.GetLabels()
	if labels["kubernetes.io/cluster-service"] == "true" ||
		labels["addonmanager.kubernetes.io/mode"] == "EnsureExists" ||
		labels["addonmanager.kubernetes.io/mode"] == "Reconcile" {
		return true
	}
	return false
}

// --- /add ons

// Cluster is a handle to a Kubernetes API server.
// (Typically, this code is deployed into the same cluster.)
type Cluster struct {
	// Do garbage collection when syncing resources
	GC bool
	// dry run garbage collection without syncing
	DryGC bool

	engine            engine.GitOpsEngine
	clusterCache      cache.ClusterCache
	client            ExtendedClient
	fallbackNamespace string

	version    string // string response for the version command.
	logger     log.Logger
	sshKeyRing ssh.KeyRing

	// syncErrors keeps a record of all per-resource errors during
	// the sync from Git repo to the cluster.
	syncErrors   map[resource.ID]error
	muSyncErrors sync.RWMutex

	allowedNamespaces map[string]struct{}

	imageIncluder cluster.Includer

	workloadsLock sync.RWMutex
	workloadsById map[resource.ID]workload

	allowedAndExistingNamespacesLock   sync.RWMutex
	allowedAndExistingNamespacesByName map[string]v1.Namespace
}

type resourceFilter struct {
	exclude []string
}

func (f *resourceFilter) IsExcludedResource(group, kind, _ string) bool {
	fullName := fmt.Sprintf("%s/%s", group, kind)
	for _, exp := range f.exclude {
		if glob.Glob(exp, fullName) {
			return true
		}
	}
	return false
}

// NewCluster returns a usable cluster.
func NewCluster(config *rest.Config, defaultNamespace string, client ExtendedClient, sshKeyRing ssh.KeyRing, logger log.Logger, allowedNamespaces []string, imageIncluder cluster.Includer, resourceExcludeList []string) (*Cluster, error) {
	fallbackNamespace, err := getFallbackNamespace(defaultNamespace)
	if err != nil {
		return nil, err
	}

	cacheSettings := cache.Settings{ResourcesFilter: &resourceFilter{exclude: resourceExcludeList}}
	// TODO: shouldn't the cache and engine have a Stop() method
	clusterCache := cache.NewClusterCache(cacheSettings, config, allowedNamespaces, &kube.KubectlCmd{})
	allowedNamespacesSet := map[string]struct{}{}
	for _, nsName := range allowedNamespaces {
		allowedNamespacesSet[nsName] = struct{}{}
	}
	c := &Cluster{
		fallbackNamespace:                  fallbackNamespace,
		engine:                             engine.NewEngine(config, clusterCache),
		clusterCache:                       clusterCache,
		client:                             client,
		logger:                             logger,
		sshKeyRing:                         sshKeyRing,
		allowedNamespaces:                  allowedNamespacesSet,
		imageIncluder:                      imageIncluder,
		workloadsById:                      map[resource.ID]workload{},
		allowedAndExistingNamespacesByName: map[string]v1.Namespace{},
	}
	clusterCache.SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, isRoot bool) (info interface{}, cacheManifest bool) {
		c.OnResourceUpdated(un)
		labels := un.GetLabels()
		if labels != nil {
			_, ok := labels[gcMarkLabel]
			return nil, ok
		}
		return nil, false
	})
	_ = clusterCache.OnResourceUpdated(func(newRes *cache.Resource, oldRes *cache.Resource, namespaceResources map[kube.ResourceKey]*cache.Resource) {
		if newRes == nil && oldRes != nil {
			c.OnResourceRemoved(oldRes.ResourceKey())
		}
	})

	return c, nil
}

func (c *Cluster) Namespacer() namespacer {
	return &namespacerViaInfoProvider{fallbackNamespace: c.fallbackNamespace, infoProvider: c.clusterCache}
}

// --- cluster.Cluster
func (c *Cluster) Run() (io.Closer, error) {
	return c.engine.Run()
}

func (c *Cluster) OnResourceRemoved(key kube.ResourceKey) {
	// remove workload
	c.workloadsLock.Lock()
	defer c.workloadsLock.Unlock()
	delete(c.workloadsById, resource.MakeID(key.Namespace, key.Kind, key.Name))

	// remove namespace
	if key.Kind == "Namespace" && key.Group == "" {
		c.allowedAndExistingNamespacesLock.Lock()
		delete(c.allowedAndExistingNamespacesByName, key.Name)
		c.allowedAndExistingNamespacesLock.Unlock()
	}
}

func (c *Cluster) OnResourceUpdated(un *unstructured.Unstructured) {
	id := resource.MakeID(un.GetNamespace(), un.GetKind(), un.GetName())

	// Update workload
	if kind, ok := resourceKinds[strings.ToLower(un.GetKind())]; ok {
		c.workloadsLock.Lock()
		defer c.workloadsLock.Unlock()
		w, err := kind.getWorkload(un)
		if err != nil {
			c.logger.Log("msg", "unable to get workflow", "kind", un.GetKind(), "err", err)
		} else {
			c.workloadsById[id] = w
		}
	}

	// Update namespace
	namespace := v1.Namespace{}
	if un.GetKind() == "Namespace" && un.GroupVersionKind().Group == "" {
		if _, ok := c.allowedNamespaces[un.GetName()]; len(c.allowedNamespaces) == 0 || ok {
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(un.Object, &namespace)
			if err != nil {
				c.logger.Log("msg", "unable to convert namespace", "err", err)
			} else {
				c.allowedAndExistingNamespacesLock.Lock()
				c.allowedAndExistingNamespacesByName[namespace.Name] = namespace
				c.allowedAndExistingNamespacesLock.Unlock()
			}
		}
	}
}

// SomeWorkloads returns the workloads named, missing out any that don't
// exist in the cluster or aren't in an allowed namespace.
// They do not necessarily have to be returned in the order requested.
func (c *Cluster) SomeWorkloads(_ context.Context, ids []resource.ID) (res []cluster.Workload, err error) {
	c.workloadsLock.RLock()
	defer c.workloadsLock.RUnlock()

	var workloads []cluster.Workload
	for _, id := range ids {
		if !c.IsAllowedResource(id) {
			continue
		}

		workload, ok := c.workloadsById[id]
		if !ok {
			continue
		}

		if !isAddon(workload) {
			c.muSyncErrors.RLock()
			workload.syncError = c.syncErrors[id]
			c.muSyncErrors.RUnlock()
			workloads = append(workloads, workload.toClusterWorkload(id))
		}
	}
	return workloads, nil
}

func (c *Cluster) allWorkloads(namespace string) (map[resource.ID]workload, error) {
	if namespace != "" {
		if _, ok := c.getAllowedAndExistingNamespaces()[namespace]; !ok {
			return map[resource.ID]workload{}, nil
		}
	}

	c.workloadsLock.RLock()
	defer c.workloadsLock.RUnlock()

	allworkloads := map[resource.ID]workload{}
	for id, workload := range c.workloadsById {
		if !isAddon(workload) && namespace == "" || workload.GetNamespace() == namespace {
			allworkloads[id] = workload
		}
	}
	return allworkloads, nil
}

// AllWorkloads returns all workloads in allowed namespaces matching the criteria; that is, in
// the namespace (or any namespace if that argument is empty)
func (c *Cluster) AllWorkloads(ctx context.Context, namespace string) ([]cluster.Workload, error) {
	workloads, err := c.allWorkloads(namespace)
	if err != nil {
		return nil, err
	}

	var allworkloads []cluster.Workload
	for id, workload := range workloads {
		c.muSyncErrors.RLock()
		workload.syncError = c.syncErrors[id]
		c.muSyncErrors.RUnlock()
		allworkloads = append(allworkloads, workload.toClusterWorkload(id))
	}

	return allworkloads, nil
}

func (c *Cluster) Ping() error {
	_, err := c.client.coreClient.Discovery().ServerVersion()
	return err
}

// Export exports cluster resources
func (c *Cluster) Export(ctx context.Context) ([]byte, error) {
	var config bytes.Buffer

	namespacesByName := c.getAllowedAndExistingNamespaces()

	encoder := yaml.NewEncoder(&config)
	defer encoder.Close()

	for _, namespace := range namespacesByName {

		// kind & apiVersion must be set, since TypeMeta is not populated
		namespace.Kind = "Namespace"
		namespace.APIVersion = "v1"

		err := encoder.Encode(yamlThroughJSON{namespace})
		if err != nil {
			return nil, errors.Wrap(err, "marshalling namespace to YAML")
		}
		workloads, err := c.allWorkloads(namespace.Name)
		if err != nil {
			return nil, err
		}

		for _, pc := range workloads {
			if err := encoder.Encode(yamlThroughJSON{pc.k8sObject}); err != nil {
				return nil, err
			}
		}
	}
	return config.Bytes(), nil
}

func (c *Cluster) PublicSSHKey(regenerate bool) (ssh.PublicKey, error) {
	if regenerate {
		if err := c.sshKeyRing.Regenerate(); err != nil {
			return ssh.PublicKey{}, err
		}
	}
	publicKey, _ := c.sshKeyRing.KeyPair()
	return publicKey, nil
}

// getAllowedAndExistingNamespaces returns a list of existing namespaces that
// the Flux instance is expected to have access to and can look for resources inside of.
// It returns a list of all namespaces unless an explicit list of allowed namespaces
// has been set on the Cluster instance.
func (c *Cluster) getAllowedAndExistingNamespaces() map[string]v1.Namespace {
	result := map[string]v1.Namespace{}
	c.allowedAndExistingNamespacesLock.RLock()
	defer c.allowedAndExistingNamespacesLock.RUnlock()
	for name, ns := range c.allowedAndExistingNamespacesByName {
		result[name] = ns
	}
	return result
}

func (c *Cluster) IsAllowedResource(id resource.ID) bool {
	if len(c.allowedNamespaces) == 0 {
		// All resources are allowed when all namespaces are allowed
		return true
	}

	namespace, kind, name := id.Components()
	namespaceToCheck := namespace

	if namespace == kresource.ClusterScope {
		// All cluster-scoped resources (not namespaced) are allowed ...
		if kind != "namespace" {
			return true
		}
		// ... except namespaces themselves, whose name needs to be explicitly allowed
		namespaceToCheck = name
	}

	_, ok := c.allowedNamespaces[namespaceToCheck]
	return ok
}

type yamlThroughJSON struct {
	toMarshal interface{}
}

func (y yamlThroughJSON) MarshalYAML() (interface{}, error) {
	rawJSON, err := json.Marshal(y.toMarshal)
	if err != nil {
		return nil, fmt.Errorf("error marshaling into JSON: %s", err)
	}
	var jsonObj interface{}
	if err = yaml.Unmarshal(rawJSON, &jsonObj); err != nil {
		return nil, fmt.Errorf("error unmarshaling from JSON: %s", err)
	}
	return jsonObj, nil
}
