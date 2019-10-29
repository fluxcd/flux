package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/argoproj/argo-cd/engine/util/kube"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	hrclient "github.com/fluxcd/helm-operator/pkg/client/clientset/versioned"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	k8sclient "k8s.io/client-go/kubernetes"

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
	discoveryClient
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
	client ExtendedClient

	version    string // string response for the version command.
	logger     log.Logger
	sshKeyRing ssh.KeyRing

	// syncErrors keeps a record of all per-resource errors during
	// the sync from Git repo to the cluster.
	syncErrors   map[resource.ID]error
	muSyncErrors sync.RWMutex

	allowedNamespaces []string
	loggedAllowedNS   map[string]bool // to keep track of whether we've logged a problem with seeing an allowed namespace

	imageExcludeList []string

	workloadsLock sync.Mutex
	workloadsById map[resource.ID]workload
}

// NewCluster returns a usable cluster.
func NewCluster(client ExtendedClient, sshKeyRing ssh.KeyRing, logger log.Logger, allowedNamespaces []string, imageExcludeList []string) *Cluster {
	c := &Cluster{
		client:            client,
		logger:            logger,
		sshKeyRing:        sshKeyRing,
		allowedNamespaces: allowedNamespaces,
		loggedAllowedNS:   map[string]bool{},
		imageExcludeList:  imageExcludeList,
		workloadsById:     map[resource.ID]workload{},
	}

	return c
}

// --- cluster.Cluster

func (c *Cluster) OnResourceRemoved(key kube.ResourceKey) {
	c.workloadsLock.Lock()
	defer c.workloadsLock.Unlock()
	delete(c.workloadsById, resource.MakeID(key.Namespace, key.Kind, key.Name))
}

func (c *Cluster) OnResourceUpdated(un *unstructured.Unstructured) {
	id := resource.MakeID(un.GetNamespace(), un.GetKind(), un.GetName())
	if kind, ok := resourceKinds[strings.ToLower(un.GetKind())]; ok {
		c.workloadsLock.Lock()
		defer c.workloadsLock.Unlock()
		w, err := kind.getWorkload(un)
		if err != nil {
			c.logger.Log("Unable to get workflow for kind", un.GetKind(), err)
		}
		c.workloadsById[id] = w
	}
}

// SomeWorkloads returns the workloads named, missing out any that don't
// exist in the cluster or aren't in an allowed namespace.
// They do not necessarily have to be returned in the order requested.
func (c *Cluster) SomeWorkloads(ctx context.Context, ids []resource.ID) (res []cluster.Workload, err error) {
	c.workloadsLock.Lock()
	defer c.workloadsLock.Unlock()

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

func (c *Cluster) allWorkfloads(ctx context.Context, namespace string) (map[resource.ID]workload, error) {
	if namespace != "" {
		namespaces, err := c.getAllowedAndExistingNamespaces(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "getting namespaces")
		}
		found := false

		for _, ns := range namespaces {
			if ns == "" || ns == namespace {
				found = true
				break
			}
		}
		if !found {
			return map[resource.ID]workload{}, nil
		}

	}

	c.workloadsLock.Lock()
	defer c.workloadsLock.Unlock()

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
	workloads, err := c.allWorkfloads(ctx, namespace)
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

	namespaces, err := c.getAllowedAndExistingNamespaces(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "getting namespaces")
	}

	encoder := yaml.NewEncoder(&config)
	defer encoder.Close()

	for _, ns := range namespaces {
		namespace, err := c.client.CoreV1().Namespaces().Get(ns, meta_v1.GetOptions{})
		if err != nil {
			return nil, err
		}

		// kind & apiVersion must be set, since TypeMeta is not populated
		namespace.Kind = "Namespace"
		namespace.APIVersion = "v1"

		err = encoder.Encode(yamlThroughJSON{namespace})
		if err != nil {
			return nil, errors.Wrap(err, "marshalling namespace to YAML")
		}
		workloads, err := c.allWorkfloads(ctx, ns)
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
func (c *Cluster) getAllowedAndExistingNamespaces(ctx context.Context) ([]string, error) {
	if len(c.allowedNamespaces) > 0 {
		nsList := []string{}
		for _, name := range c.allowedNamespaces {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			ns, err := c.client.CoreV1().Namespaces().Get(name, meta_v1.GetOptions{})
			switch {
			case err == nil:
				c.loggedAllowedNS[name] = false // reset, so if the namespace goes away we'll log it again
				nsList = append(nsList, ns.Name)
			case apierrors.IsUnauthorized(err) || apierrors.IsForbidden(err) || apierrors.IsNotFound(err):
				if !c.loggedAllowedNS[name] {
					c.logger.Log("warning", "cannot access allowed namespace",
						"namespace", name, "err", err)
					c.loggedAllowedNS[name] = true
				}
			default:
				return nil, err
			}
		}
		return nsList, nil
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []string{meta_v1.NamespaceAll}, nil
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

	for _, allowedNS := range c.allowedNamespaces {
		if namespaceToCheck == allowedNS {
			return true
		}
	}
	return false
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
