package kubernetes

import (
	"bytes"
	"fmt"

	k8syaml "github.com/ghodss/yaml"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	k8sclient "k8s.io/client-go/kubernetes"
	v1beta1apps "k8s.io/client-go/kubernetes/typed/apps/v1beta1"
	v2alpha1batch "k8s.io/client-go/kubernetes/typed/batch/v2alpha1"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	v1beta1extensions "k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
	"k8s.io/client-go/pkg/api"
	apiv1 "k8s.io/client-go/pkg/api/v1"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/registry"
	"github.com/weaveworks/flux/ssh"
)

const (
	StatusUnknown  = "unknown"
	StatusReady    = "ready"
	StatusUpdating = "updating"
)

type extendedClient struct {
	discovery.DiscoveryInterface
	v1core.CoreV1Interface
	v1beta1extensions.ExtensionsV1beta1Interface
	v1beta1apps.StatefulSetsGetter
	v2alpha1batch.CronJobsGetter
}

type apiObject struct {
	bytes    []byte
	Version  string `yaml:"apiVersion"`
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
}

func (obj *apiObject) namespaceOrDefault() string {
	if obj.Metadata.Namespace == "" {
		return "default"
	}
	return obj.Metadata.Namespace
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

type namespacedLabeled interface {
	GetNamespace() string
	GetLabels() map[string]string
}

func isAddon(obj namespacedLabeled) bool {
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

type Applier interface {
	Delete(logger log.Logger, def *apiObject) error
	Apply(logger log.Logger, def *apiObject) error
}

// Cluster is a handle to a Kubernetes API server.
// (Typically, this code is deployed into the same cluster.)
type Cluster struct {
	client     extendedClient
	applier    Applier
	actionc    chan func()
	version    string // string response for the version command.
	logger     log.Logger
	sshKeyRing ssh.KeyRing
}

// NewCluster returns a usable cluster. Host should be of the form
// "http://hostname:8080".
func NewCluster(clientset k8sclient.Interface,
	applier Applier,
	sshKeyRing ssh.KeyRing,
	logger log.Logger) (*Cluster, error) {

	c := &Cluster{
		client: extendedClient{
			clientset.Discovery(),
			clientset.Core(),
			clientset.Extensions(),
			clientset.AppsV1beta1(),
			clientset.BatchV2alpha1()},
		applier:    applier,
		actionc:    make(chan func()),
		logger:     logger,
		sshKeyRing: sshKeyRing,
	}

	go c.loop()
	return c, nil
}

// Stop terminates the goroutine that serializes and executes requests against
// the cluster. A stopped cluster cannot be restarted.
func (c *Cluster) Stop() {
	close(c.actionc)
}

func (c *Cluster) loop() {
	for f := range c.actionc {
		f()
	}
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
			controllers = append(controllers, podController.toClusterController(id))
		}
	}
	return controllers, nil
}

// AllControllers returns all controllers matching the criteria; that is, in
// the namespace (or any namespace if that argument is empty)
func (c *Cluster) AllControllers(namespace string) (res []cluster.Controller, err error) {
	namespaces, err := c.client.Namespaces().List(meta_v1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "getting namespaces")
	}

	var allControllers []cluster.Controller
	for _, ns := range namespaces.Items {
		if namespace != "" && ns.Name != namespace {
			continue
		}

		for kind, resourceKind := range resourceKinds {
			podControllers, err := resourceKind.getPodControllers(c, ns.Name)
			if err != nil {
				if se, ok := err.(*apierrors.StatusError); ok && se.ErrStatus.Reason == meta_v1.StatusReasonNotFound {
					// Kind not supported by API server, skip
					continue
				} else {
					return nil, err
				}
			}

			for _, podController := range podControllers {
				if !isAddon(podController) {
					id := flux.MakeResourceID(ns.Name, kind, podController.name)
					allControllers = append(allControllers, podController.toClusterController(id))
				}
			}
		}
	}

	return allControllers, nil
}

// Sync performs the given actions on resources. Operations are
// asynchronous, but serialised.
func (c *Cluster) Sync(spec cluster.SyncDef) error {
	errc := make(chan error)
	logger := log.With(c.logger, "method", "Sync")
	c.actionc <- func() {
		errs := cluster.SyncError{}
		for _, action := range spec.Actions {
			logger := log.With(logger, "resource", action.ResourceID)
			if len(action.Delete) > 0 {
				obj, err := definitionObj(action.Delete)
				if err == nil {
					err = c.applier.Delete(logger, obj)
				}
				if err != nil {
					errs[action.ResourceID] = err
					continue
				}
			}
			if len(action.Apply) > 0 {
				obj, err := definitionObj(action.Apply)
				if err == nil {
					err = c.applier.Apply(logger, obj)
				}
				if err != nil {
					errs[action.ResourceID] = err
					continue
				}
			}
		}
		if len(errs) > 0 {
			errc <- errs
		} else {
			errc <- nil
		}
	}
	return <-errc
}

func (c *Cluster) Ping() error {
	_, err := c.client.ServerVersion()
	return err
}

// Export exports cluster resources
func (c *Cluster) Export() ([]byte, error) {
	var config bytes.Buffer
	list, err := c.client.Namespaces().List(meta_v1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "getting namespaces")
	}
	for _, ns := range list.Items {
		err := appendYAML(&config, "v1", "Namespace", ns)
		if err != nil {
			return nil, errors.Wrap(err, "marshalling namespace to YAML")
		}

		for _, resourceKind := range resourceKinds {
			podControllers, err := resourceKind.getPodControllers(c, ns.Name)
			if err != nil {
				if se, ok := err.(*apierrors.StatusError); ok && se.ErrStatus.Reason == meta_v1.StatusReasonNotFound {
					// Kind not supported by API server, skip
					continue
				} else {
					return nil, err
				}
			}

			for _, pc := range podControllers {
				if !isAddon(pc) {
					if err := appendYAML(&config, pc.apiVersion, pc.kind, pc.apiObject); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	return config.Bytes(), nil
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

func mergeCredentials(c *Cluster, namespace string, podTemplate apiv1.PodTemplateSpec, imageCreds registry.ImageCreds) {
	creds := registry.NoCredentials()
	for _, imagePullSecret := range podTemplate.Spec.ImagePullSecrets {
		secret, err := c.client.Secrets(namespace).Get(imagePullSecret.Name, meta_v1.GetOptions{})
		if err != nil {
			c.logger.Log("err", errors.Wrapf(err, "getting secret %q from namespace %q", secret.Name, namespace))
			continue
		}

		var decoded []byte
		var ok bool
		// These differ in format; but, ParseCredentials will
		// handle either.
		switch api.SecretType(secret.Type) {
		case api.SecretTypeDockercfg:
			decoded, ok = secret.Data[api.DockerConfigKey]
		case api.SecretTypeDockerConfigJson:
			decoded, ok = secret.Data[api.DockerConfigJsonKey]
		default:
			c.logger.Log("skip", "unknown type", "secret", namespace+"/"+secret.Name, "type", secret.Type)
			continue
		}

		if !ok {
			c.logger.Log("err", errors.Wrapf(err, "retrieving pod secret %q", secret.Name))
			continue
		}

		// Parse secret
		crd, err := registry.ParseCredentials(decoded)
		if err != nil {
			c.logger.Log("err", err.Error())
			continue
		}

		// Merge into the credentials for this PodSpec
		creds.Merge(crd)
	}

	// Now create the service and attach the credentials
	for _, container := range podTemplate.Spec.Containers {
		r, err := flux.ParseImageID(container.Image)
		if err != nil {
			c.logger.Log("err", err.Error())
			continue
		}
		imageCreds[r] = creds
	}
}

// ImagesToFetch is a k8s specific method to get a list of images to update along with their credentials
func (c *Cluster) ImagesToFetch() registry.ImageCreds {
	allImageCreds := make(registry.ImageCreds)

	namespaces, err := c.client.Namespaces().List(meta_v1.ListOptions{})
	if err != nil {
		c.logger.Log("err", errors.Wrap(err, "getting namespaces"))
		return allImageCreds
	}

	for _, ns := range namespaces.Items {
		for kind, resourceKind := range resourceKinds {
			podControllers, err := resourceKind.getPodControllers(c, ns.Name)
			if err != nil {
				if se, ok := err.(*apierrors.StatusError); ok && se.ErrStatus.Reason == meta_v1.StatusReasonNotFound {
					// Kind not supported by API server, skip
				} else {
					c.logger.Log("err", errors.Wrapf(err, "getting kind %s for namespace %s", kind, ns.Name))
				}
				continue
			}

			imageCreds := make(registry.ImageCreds)
			for _, podController := range podControllers {
				mergeCredentials(c, ns.Name, podController.podTemplate, imageCreds)
			}

			// Merge creds
			for imageID, creds := range imageCreds {
				existingCreds, ok := allImageCreds[imageID]
				if ok {
					existingCreds.Merge(creds)
				} else {
					allImageCreds[imageID] = creds
				}
			}
		}
	}

	return allImageCreds
}

// --- end cluster.Cluster

// A convenience for getting an minimal object from some bytes.
func definitionObj(bytes []byte) (*apiObject, error) {
	obj := apiObject{bytes: bytes}
	return &obj, yaml.Unmarshal(bytes, &obj)
}
