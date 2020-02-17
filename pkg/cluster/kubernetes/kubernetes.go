package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	hrclient "github.com/fluxcd/helm-operator/pkg/client/clientset/versioned"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	k8sclientdynamic "k8s.io/client-go/dynamic"
	k8sclient "k8s.io/client-go/kubernetes"

	fhrclient "github.com/fluxcd/flux/integrations/client/clientset/versioned"
	"github.com/fluxcd/flux/pkg/cluster"
	kresource "github.com/fluxcd/flux/pkg/cluster/kubernetes/resource"
	"github.com/fluxcd/flux/pkg/resource"
	"github.com/fluxcd/flux/pkg/ssh"
)

type coreClient k8sclient.Interface
type dynamicClient k8sclientdynamic.Interface
type fluxHelmClient fhrclient.Interface
type helmOperatorClient hrclient.Interface
type discoveryClient discovery.DiscoveryInterface

type ExtendedClient struct {
	coreClient
	dynamicClient
	fluxHelmClient
	helmOperatorClient
	discoveryClient
}

func MakeClusterClientset(core coreClient, dyn dynamicClient, fluxhelm fluxHelmClient,
	helmop helmOperatorClient, disco discoveryClient) ExtendedClient {

	return ExtendedClient{
		coreClient:         core,
		dynamicClient:      dyn,
		fluxHelmClient:     fluxhelm,
		helmOperatorClient: helmop,
		discoveryClient:    disco,
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

	client  ExtendedClient
	applier Applier

	version    string // string response for the version command.
	logger     log.Logger
	sshKeyRing ssh.KeyRing

	// syncErrors keeps a record of all per-resource errors during
	// the sync from Git repo to the cluster.
	syncErrors   map[resource.ID]error
	muSyncErrors sync.RWMutex

	allowedNamespaces map[string]struct{}
	loggedAllowedNS   map[string]bool // to keep track of whether we've logged a problem with seeing an allowed namespace

	imageIncluder       cluster.Includer
	resourceExcludeList []string
	mu                  sync.Mutex
}

// NewCluster returns a usable cluster.
func NewCluster(client ExtendedClient, applier Applier, sshKeyRing ssh.KeyRing, logger log.Logger, allowedNamespaces map[string]struct{}, imageIncluder cluster.Includer, resourceExcludeList []string) *Cluster {
	if imageIncluder == nil {
		imageIncluder = cluster.AlwaysInclude
	}

	c := &Cluster{
		client:              client,
		applier:             applier,
		logger:              logger,
		sshKeyRing:          sshKeyRing,
		allowedNamespaces:   allowedNamespaces,
		loggedAllowedNS:     map[string]bool{},
		imageIncluder:       imageIncluder,
		resourceExcludeList: resourceExcludeList,
	}

	return c
}

// --- cluster.Cluster

// SomeWorkloads returns the workloads named, missing out any that don't
// exist in the cluster or aren't in an allowed namespace.
// They do not necessarily have to be returned in the order requested.
func (c *Cluster) SomeWorkloads(ctx context.Context, ids []resource.ID) (res []cluster.Workload, err error) {
	var workloads []cluster.Workload
	for _, id := range ids {
		if !c.IsAllowedResource(id) {
			continue
		}
		ns, kind, name := id.Components()

		resourceKind, ok := resourceKinds[kind]
		if !ok {
			c.logger.Log("warning", "automation of this resource kind is not supported", "resource", id)
			continue
		}

		workload, err := resourceKind.getWorkload(ctx, c, ns, name)
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
func (c *Cluster) AllWorkloads(ctx context.Context, restrictToNamespace string) (res []cluster.Workload, err error) {
	allowedNamespaces, err := c.getAllowedAndExistingNamespaces(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "getting namespaces")
	}
	// Those are the allowed namespaces (possibly just [<all of them>];
	// now intersect with the restriction requested, if any.
	namespaces := allowedNamespaces
	if restrictToNamespace != "" {
		namespaces = nil
		for _, ns := range allowedNamespaces {
			if ns == meta_v1.NamespaceAll || ns == restrictToNamespace {
				namespaces = []string{restrictToNamespace}
				break
			}
		}
	}

	var allworkloads []cluster.Workload
	for _, ns := range namespaces {
		for kind, resourceKind := range resourceKinds {
			workloads, err := resourceKind.getWorkloads(ctx, c, ns)
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
					id := resource.MakeID(workload.GetNamespace(), kind, workload.GetName())
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
	c.syncErrors = make(map[resource.ID]error)
	for _, e := range errs {
		c.syncErrors[e.ResourceID] = e.Error
	}
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

		for _, resourceKind := range resourceKinds {
			workloads, err := resourceKind.getWorkloads(ctx, c, ns)
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
					if err := encoder.Encode(yamlThroughJSON{pc.k8sObject}); err != nil {
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
func (c *Cluster) getAllowedAndExistingNamespaces(ctx context.Context) ([]string, error) {
	if len(c.allowedNamespaces) > 0 {
		nsList := []string{}
		for name, _ := range c.allowedNamespaces {
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
