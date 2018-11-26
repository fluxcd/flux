package kubernetes

import (
	"bytes"
	"fmt"
	"strings"
	"sync"

	k8syaml "github.com/ghodss/yaml"
	"github.com/go-kit/kit/log"
	"github.com/imdario/mergo"
	"github.com/pkg/errors"
	kresource "github.com/weaveworks/flux/cluster/kubernetes/resource"
	fhrclient "github.com/weaveworks/flux/integrations/client/clientset/versioned"
	"gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sclientdynamic "k8s.io/client-go/dynamic"
	k8sclient "k8s.io/client-go/kubernetes"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/resource"
	"github.com/weaveworks/flux/ssh"
)

type coreClient k8sclient.Interface
type dynamicClient k8sclientdynamic.Interface
type fluxHelmClient fhrclient.Interface

type extendedClient struct {
	coreClient
	dynamicClient
	fluxHelmClient
}

const (
	stackLabel         = kresource.PolicyPrefix + "stack"
	checksumAnnotation = kresource.PolicyPrefix + "stack_checksum"
)

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

	client     extendedClient
	applier    Applier
	version    string // string response for the version command.
	logger     log.Logger
	sshKeyRing ssh.KeyRing

	// syncErrors keeps a record of all per-resource errors during
	// the sync from Git repo to the cluster.
	syncErrors   map[flux.ResourceID]error
	muSyncErrors sync.RWMutex

	nsWhitelist       []string
	nsWhitelistLogged map[string]bool // to keep track of whether we've logged a problem with seeing a whitelisted ns

	imageExcludeList []string
	mu               sync.Mutex
}

// NewCluster returns a usable cluster.
func NewCluster(clientset k8sclient.Interface,
	dynamicClientset k8sclientdynamic.Interface,
	fluxHelmClientset fhrclient.Interface,
	applier Applier,
	sshKeyRing ssh.KeyRing,
	logger log.Logger,
	nsWhitelist []string,
	imageExcludeList []string) *Cluster {

	c := &Cluster{
		client: extendedClient{
			clientset,
			dynamicClientset,
			fluxHelmClientset,
		},
		applier:           applier,
		logger:            logger,
		sshKeyRing:        sshKeyRing,
		nsWhitelist:       nsWhitelist,
		nsWhitelistLogged: map[string]bool{},
		imageExcludeList:  imageExcludeList,
	}

	return c
}

// --- cluster.Cluster

// SomeControllers returns the controllers named, missing out any that don't
// exist in the cluster. They do not necessarily have to be returned
// in the order requested.
func (c *Cluster) SomeControllers(ids []flux.ResourceID) (res []cluster.Controller, err error) {
	var controllers []cluster.Controller
	for _, id := range ids {
		ns, kind, name := id.Components()

		resourceKind, ok := resourceKinds[kind]
		if !ok {
			return nil, fmt.Errorf("Unsupported kind %v", kind)
		}

		podController, err := resourceKind.getPodController(c, ns, name)
		if err != nil {
			return nil, err
		}

		if !isAddon(podController) {
			c.muSyncErrors.RLock()
			podController.syncError = c.syncErrors[id]
			c.muSyncErrors.RUnlock()
			controllers = append(controllers, podController.toClusterController(id))
		}
	}
	return controllers, nil
}

// AllControllers returns all controllers matching the criteria; that is, in
// the namespace (or any namespace if that argument is empty)
func (c *Cluster) AllControllers(namespace string) (res []cluster.Controller, err error) {
	namespaces, err := c.getAllowedNamespaces()
	if err != nil {
		return nil, errors.Wrap(err, "getting namespaces")
	}

	var allControllers []cluster.Controller
	for _, ns := range namespaces {
		if namespace != "" && ns.Name != namespace {
			continue
		}

		for kind, resourceKind := range resourceKinds {
			podControllers, err := resourceKind.getPodControllers(c, ns.Name)
			if err != nil {
				if se, ok := err.(*apierrors.StatusError); ok {
					switch se.ErrStatus.Reason {
					case meta_v1.StatusReasonNotFound:
						// Kind not supported by API server, skip
						continue
					case meta_v1.StatusReasonForbidden:
						// K8s can return forbidden instead of not found for non super admins
						c.logger.Log("warning", "not allowed to list resources", "err", err)
						continue
					default:
						return nil, err
					}
				} else {
					return nil, err
				}
			}

			for _, podController := range podControllers {
				if !isAddon(podController) {
					id := flux.MakeResourceID(ns.Name, kind, podController.name)
					c.muSyncErrors.RLock()
					podController.syncError = c.syncErrors[id]
					c.muSyncErrors.RUnlock()
					allControllers = append(allControllers, podController.toClusterController(id))
				}
			}
		}
	}

	return allControllers, nil
}

func applyMetadata(res resource.Resource, stack, checksum string) ([]byte, error) {
	definition := map[interface{}]interface{}{}
	if err := yaml.Unmarshal(res.Bytes(), &definition); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to parse yaml from %s", res.Source()))
	}

	mixin := map[string]interface{}{}

	if stack != "" {
		mixinLabels := map[string]string{}
		mixinLabels[stackLabel] = stack
		mixin["labels"] = mixinLabels
	}

	if checksum != "" {
		mixinAnnotations := map[string]string{}
		mixinAnnotations[checksumAnnotation] = checksum
		mixin["annotations"] = mixinAnnotations
	}

	mergo.Merge(&definition, map[interface{}]interface{}{
		"metadata": mixin,
	})

	bytes, err := yaml.Marshal(definition)
	if err != nil {
		return nil, errors.Wrap(err, "failed to serialize yaml after applying metadata")
	}
	return bytes, nil
}

// Sync takes a definition of what should be running in the cluster,
// and attempts to make the cluster conform. An error return does not
// necessarily indicate complete failure; some resources may succeed
// in being synced, and some may fail (for example, they may be
// malformed).
func (c *Cluster) Sync(spec cluster.SyncDef) error {
	logger := log.With(c.logger, "method", "Sync")

	// Keep track of the checksum of each stack, so we can compare
	// them during garbage collection.
	checksums := map[string]string{}

	cs := makeChangeSet()
	var errs cluster.SyncError
	for _, stack := range spec.Stacks {
		checksums[stack.Name] = stack.Checksum
		for _, res := range stack.Resources {
			resBytes, err := applyMetadata(res, stack.Name, stack.Checksum)
			if err == nil {
				cs.stage("apply", res.ResourceID(), res.Source(), resBytes)
			} else {
				errs = append(errs, cluster.ResourceError{ResourceID: res.ResourceID(), Source: res.Source(), Error: err})
				break
			}
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.muSyncErrors.RLock()
	if applyErrs := c.applier.apply(logger, cs, c.syncErrors); len(applyErrs) > 0 {
		errs = append(errs, applyErrs...)
	}
	c.muSyncErrors.RUnlock()

	if c.GC {
		orphanedResources := makeChangeSet()

		clusterResources, err := c.getResourcesInStack()
		if err != nil {
			return errors.Wrap(err, "exporting resource defs from cluster for garbage collection")
		}

		for resourceID, res := range clusterResources {
			stack := res.GetStack()
			if stack == "" {
				c.logger.Log("warning", "cluster resource with empty stack label; skipping", "resource", resourceID)
				continue
			}

			expected := checksums[stack] // shall be "" if no such resource was applied earlier
			actual := res.GetChecksum()
			switch {
			case expected == "":
				c.logger.Log("info", "cluster resource was not applied this sync; deleting", "resource", resourceID, "actual", actual, "stack", stack)
				orphanedResources.stage("delete", res.ResourceID(), "<cluster>", res.Bytes())
			case actual != expected: // including if checksum is ""
				c.logger.Log("warning", "cluster resource has out-of-date checksum; deleting", "resource", resourceID, "actual", actual, "expected", expected)
				orphanedResources.stage("delete", res.ResourceID(), "<cluster>", res.Bytes())
			default:
				// the checksum is the same, indicating that it was applied earlier. Leave it alone.
			}
		}

		if deleteErrs := c.applier.apply(logger, orphanedResources, nil); len(deleteErrs) > 0 {
			errs = append(errs, deleteErrs...)
		}
	}

	// If `nil`, errs is a cluster.SyncError(nil) rather than error(nil), so it cannot be returned directly.
	if errs == nil {
		return nil
	}

	// It is expected that Cluster.Sync is invoked with *all* resources.
	// Otherwise it will override previously recorded sync errors.
	c.setSyncErrors(errs)
	return errs
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

	namespaces, err := c.getAllowedNamespaces()
	if err != nil {
		return nil, errors.Wrap(err, "getting namespaces")
	}

	for _, ns := range namespaces {
		err := appendYAML(&config, "v1", "Namespace", ns)
		if err != nil {
			return nil, errors.Wrap(err, "marshalling namespace to YAML")
		}

		for _, resourceKind := range resourceKinds {
			podControllers, err := resourceKind.getPodControllers(c, ns.Name)
			if err != nil {
				if se, ok := err.(*apierrors.StatusError); ok {
					switch se.ErrStatus.Reason {
					case meta_v1.StatusReasonNotFound:
						// Kind not supported by API server, skip
						continue
					case meta_v1.StatusReasonForbidden:
						// K8s can return forbidden instead of not found for non super admins
						c.logger.Log("warning", "not allowed to list resources", "err", err)
						continue
					default:
						return nil, err
					}
				} else {
					return nil, err
				}
			}

			for _, pc := range podControllers {
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

func contains(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}

type kuberesource struct {
	obj *unstructured.Unstructured
}

func (r *kuberesource) ResourceID() flux.ResourceID {
	ns, kind, name := r.obj.GetNamespace(), r.obj.GetKind(), r.obj.GetName()
	return flux.MakeResourceID(ns, kind, name)
}

// Bytes returns a byte slice description
func (r *kuberesource) Bytes() []byte {
	return []byte(fmt.Sprintf(`
apiVersion: %s
kind: %s
metadata:
  namespace: %q
  name: %q
`, r.obj.GetAPIVersion(), r.obj.GetKind(), r.obj.GetNamespace(), r.obj.GetName()))
}

// GetChecksum returns the checksum recorded on the resource from
// Kubernetes, or an empty string if it's not present.
func (r *kuberesource) GetChecksum() string {
	return r.obj.GetAnnotations()[checksumAnnotation]
}

// GetStack returns the stack recorded on the the resource from
// Kubernetes, or an empty string if it's not present.
func (r *kuberesource) GetStack() string {
	return r.obj.GetLabels()[stackLabel]
}

// exportResourcesInStack collates all the resources that belong to a
// stack, i.e., were applied by flux.
func (c *Cluster) getResourcesInStack() (map[string]*kuberesource, error) {
	resources, err := c.client.coreClient.Discovery().ServerResources()
	if err != nil {
		return nil, err
	}

	result := map[string]*kuberesource{}

	for _, resource := range resources {
		for _, apiResource := range resource.APIResources {
			verbs := apiResource.Verbs
			// skip resources that can't be listed
			if !contains(verbs, "list") {
				continue
			}

			// get group and version
			var group, version string
			groupVersion := resource.GroupVersion
			if strings.Contains(groupVersion, "/") {
				a := strings.SplitN(groupVersion, "/", 2)
				group = a[0]
				version = a[1]
			} else {
				group = ""
				version = groupVersion
			}

			resourceClient := c.client.dynamicClient.Resource(schema.GroupVersionResource{
				Group:    group,
				Version:  version,
				Resource: apiResource.Name,
			})
			data, err := resourceClient.List(meta_v1.ListOptions{
				LabelSelector: stackLabel, // means "has label <<stackLabel>>"
			})
			if err != nil {
				return nil, err
			}

			for i, item := range data.Items {
				apiVersion := item.GetAPIVersion()
				kind := item.GetKind()

				itemDesc := fmt.Sprintf("%s:%s", apiVersion, kind)
				// https://github.com/kontena/k8s-client/blob/6e9a7ba1f03c255bd6f06e8724a1c7286b22e60f/lib/k8s/stack.rb#L17-L22
				if itemDesc == "v1:ComponentStatus" || itemDesc == "v1:Endpoints" {
					continue
				}
				// TODO(michael) also exclude anything that has an ownerReference (that isn't "standard"?)

				res := &kuberesource{obj: &data.Items[i]}
				result[res.ResourceID().String()] = res
			}
		}
	}

	return result, nil
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

func (c *Cluster) PublicSSHKey(regenerate bool) (ssh.PublicKey, error) {
	if regenerate {
		if err := c.sshKeyRing.Regenerate(); err != nil {
			return ssh.PublicKey{}, err
		}
	}
	publicKey, _ := c.sshKeyRing.KeyPair()
	return publicKey, nil
}

// getAllowedNamespaces returns a list of namespaces that the Flux instance is expected
// to have access to and can look for resources inside of.
// It returns a list of all namespaces unless a namespace whitelist has been set on the Cluster
// instance, in which case it returns a list containing the namespaces from the whitelist
// that exist in the cluster.
func (c *Cluster) getAllowedNamespaces() ([]apiv1.Namespace, error) {
	if len(c.nsWhitelist) > 0 {
		nsList := []apiv1.Namespace{}
		for _, name := range c.nsWhitelist {
			ns, err := c.client.CoreV1().Namespaces().Get(name, meta_v1.GetOptions{})
			switch {
			case err == nil:
				c.nsWhitelistLogged[name] = false // reset, so if the namespace goes away we'll log it again
				nsList = append(nsList, *ns)
			case apierrors.IsUnauthorized(err) || apierrors.IsForbidden(err) || apierrors.IsNotFound(err):
				if !c.nsWhitelistLogged[name] {
					c.logger.Log("warning", "whitelisted namespace inaccessible", "namespace", name, "err", err)
					c.nsWhitelistLogged[name] = true
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
