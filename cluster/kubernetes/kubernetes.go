package kubernetes

import (
	"bytes"
	"fmt"
	"sync"

	k8syaml "github.com/ghodss/yaml"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	k8sclientdynamic "k8s.io/client-go/dynamic"
	k8sclient "k8s.io/client-go/kubernetes"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/cluster/kubernetes/resource"
	fhrclient "github.com/weaveworks/flux/integrations/client/clientset/versioned"
	"github.com/weaveworks/flux/ssh"
)

type coreClient k8sclient.Interface
type dynamicClient k8sclientdynamic.Interface
type fluxHelmClient fhrclient.Interface
type discoveryClient discovery.DiscoveryInterface

type ExtendedClient struct {
	coreClient
	dynamicClient
	fluxHelmClient
	discoveryClient
}

func MakeClusterClientset(core coreClient, dyn dynamicClient, fluxhelm fluxHelmClient, disco discoveryClient) ExtendedClient {
	return ExtendedClient{
		coreClient:      core,
		dynamicClient:   dyn,
		fluxHelmClient:  fluxhelm,
		discoveryClient: disco,
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

	client  ExtendedClient
	applier Applier

	version    string // string response for the version command.
	logger     log.Logger
	sshKeyRing ssh.KeyRing

	// syncErrors keeps a record of all per-resource errors during
	// the sync from Git repo to the cluster.
	syncErrors   map[flux.ResourceID]error
	muSyncErrors sync.RWMutex

	allowedNamespaces []string
	loggedAllowedNS   map[string]bool // to keep track of whether we've logged a problem with seeing an allowed namespace

	imageExcludeList []string
	mu               sync.Mutex
}

// NewCluster returns a usable cluster.
func NewCluster(client ExtendedClient, applier Applier, sshKeyRing ssh.KeyRing, logger log.Logger, allowedNamespaces []string, imageExcludeList []string) *Cluster {
	c := &Cluster{
		client:            client,
		applier:           applier,
		logger:            logger,
		sshKeyRing:        sshKeyRing,
		allowedNamespaces: allowedNamespaces,
		loggedAllowedNS:   map[string]bool{},
		imageExcludeList:  imageExcludeList,
	}

	return c
}

// --- cluster.Cluster

// SomeWorkloads returns the workloads named, missing out any that don't
// exist in the cluster or aren't in an allowed namespace.
// They do not necessarily have to be returned in the order requested.
func (c *Cluster) SomeWorkloads(ids []flux.ResourceID) (res []cluster.Workload, err error) {
	var workloads []cluster.Workload
	for _, id := range ids {
		if !c.IsAllowedResource(id) {
			continue
		}
		ns, kind, name := id.Components()

		resourceKind, ok := resourceKinds[kind]
		if !ok {
			return nil, fmt.Errorf("Unsupported kind %v", kind)
		}

		workload, err := resourceKind.getWorkload(c, ns, name)
		if err != nil {
			if apierrors.IsForbidden(err) || apierrors.IsNotFound(err) {
				continue
			}
			return nil, err
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

// AllWorkloads returns all workloads in allowed namespaces matching the criteria; that is, in
// the namespace (or any namespace if that argument is empty)
func (c *Cluster) AllWorkloads(namespace string) (res []cluster.Workload, err error) {
	namespaces, err := c.getAllowedAndExistingNamespaces()
	if err != nil {
		return nil, errors.Wrap(err, "getting namespaces")
	}

	var allworkloads []cluster.Workload
	for _, ns := range namespaces {
		if namespace != "" && ns.Name != namespace {
			continue
		}

		for kind, resourceKind := range resourceKinds {
			workloads, err := resourceKind.getWorkloads(c, ns.Name)
			if err != nil {
				switch {
				case apierrors.IsNotFound(err):
					// Kind not supported by API server, skip
					continue
				case apierrors.IsForbidden(err):
					// K8s can return forbidden instead of not found for non super admins
					c.logger.Log("warning", "not allowed to list resources", "err", err)
					continue
				default:
					return nil, err
				}
			}

			for _, workload := range workloads {
				if !isAddon(workload) {
					id := flux.MakeResourceID(ns.Name, kind, workload.name)
					c.muSyncErrors.RLock()
					workload.syncError = c.syncErrors[id]
					c.muSyncErrors.RUnlock()
					allworkloads = append(allworkloads, workload.toClusterWorkload(id))
				}
			}
		}
	}

	return allworkloads, nil
}

func (c *Cluster) setSyncErrors(errs cluster.SyncError) {
	c.muSyncErrors.Lock()
	defer c.muSyncErrors.Unlock()
	c.syncErrors = make(map[flux.ResourceID]error)
	for _, e := range errs {
		c.syncErrors[e.ResourceID] = e.Error
	}
}

func (c *Cluster) Ping() error {
	_, err := c.client.coreClient.Discovery().ServerVersion()
	return err
}

// Export exports cluster resources
func (c *Cluster) Export() ([]byte, error) {
	var config bytes.Buffer

	namespaces, err := c.getAllowedAndExistingNamespaces()
	if err != nil {
		return nil, errors.Wrap(err, "getting namespaces")
	}

	for _, ns := range namespaces {
		err := appendYAML(&config, "v1", "Namespace", ns)
		if err != nil {
			return nil, errors.Wrap(err, "marshalling namespace to YAML")
		}

		for _, resourceKind := range resourceKinds {
			workloads, err := resourceKind.getWorkloads(c, ns.Name)
			if err != nil {
				switch {
				case apierrors.IsNotFound(err):
					// Kind not supported by API server, skip
					continue
				case apierrors.IsForbidden(err):
					// K8s can return forbidden instead of not found for non super admins
					c.logger.Log("warning", "not allowed to list resources", "err", err)
					continue
				default:
					return nil, err
				}
			}

			for _, pc := range workloads {
				if !isAddon(pc) {
					if err := appendYAML(&config, pc.apiVersion, pc.kind, pc.k8sObject); err != nil {
						return nil, err
					}
				}
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
func (c *Cluster) getAllowedAndExistingNamespaces() ([]apiv1.Namespace, error) {
	if len(c.allowedNamespaces) > 0 {
		nsList := []apiv1.Namespace{}
		for _, name := range c.allowedNamespaces {
			ns, err := c.client.CoreV1().Namespaces().Get(name, meta_v1.GetOptions{})
			switch {
			case err == nil:
				c.loggedAllowedNS[name] = false // reset, so if the namespace goes away we'll log it again
				nsList = append(nsList, *ns)
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

	namespaces, err := c.client.CoreV1().Namespaces().List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return namespaces.Items, nil
}

func (c *Cluster) IsAllowedResource(id flux.ResourceID) bool {
	if len(c.allowedNamespaces) == 0 {
		// All resources are allowed when all namespaces are allowed
		return true
	}

	namespace, kind, name := id.Components()
	namespaceToCheck := namespace

	if namespace == resource.ClusterScope {
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

// kind & apiVersion must be passed separately as the object's TypeMeta is not populated
func appendYAML(buffer *bytes.Buffer, apiVersion, kind string, object interface{}) error {
	yamlBytes, err := k8syaml.Marshal(object)
	if err != nil {
		return err
	}
	buffer.WriteString("---\n")
	buffer.WriteString("apiVersion: ")
	buffer.WriteString(apiVersion)
	buffer.WriteString("\nkind: ")
	buffer.WriteString(kind)
	buffer.WriteString("\n")
	buffer.Write(yamlBytes)
	return nil
}
