// Package kubernetes provides abstractions for the Kubernetes platform. At the
// moment, Kubernetes is the only supported platform, so we are directly
// returning Kubernetes objects. As we add more platforms, we will create
// abstractions and common data types in package platform.
package kubernetes

import (
	"bytes"
	"fmt"

	k8syaml "github.com/ghodss/yaml"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/1.5/discovery"
	k8sclient "k8s.io/client-go/1.5/kubernetes"
	v1core "k8s.io/client-go/1.5/kubernetes/typed/core/v1"
	v1beta1extensions "k8s.io/client-go/1.5/kubernetes/typed/extensions/v1beta1"
	"k8s.io/client-go/1.5/pkg/api"
	apiext "k8s.io/client-go/1.5/pkg/apis/extensions/v1beta1"

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
	v1core.CoreInterface
	v1beta1extensions.ExtensionsInterface
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
		client:     extendedClient{clientset.Discovery(), clientset.Core(), clientset.Extensions()},
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

		switch kind {
		case "deployment":
			deployment, err := c.client.Deployments(ns).Get(name)
			if err != nil {
				return nil, errors.Wrapf(err, "fetching deployment %s for namespace %S", name, ns)
			}
			if !isAddon(deployment) {
				controllers = append(controllers, makeDeploymentController(id, deployment))
			}
		}
	}
	return controllers, nil
}

// AllControllers returns all controllers matching the criteria; that is, in
// the namespace (or any namespace if that argument is empty)
func (c *Cluster) AllControllers(namespace string) (res []cluster.Controller, err error) {
	namespaces, err := c.client.Namespaces().List(api.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "getting namespaces")
	}

	var controllers []cluster.Controller
	for _, ns := range namespaces.Items {
		if namespace != "" && ns.Name != namespace {
			continue
		}

		deployments, err := c.client.Deployments(ns.Name).List(api.ListOptions{})
		if err != nil {
			return nil, errors.Wrapf(err, "getting deployments for namespace %s", ns.Name)
		}

		for _, deployment := range deployments.Items {
			if !isAddon(&deployment) {
				id := flux.MakeResourceID(ns.Name, "deployment", deployment.Name)
				controllers = append(controllers, makeDeploymentController(id, &deployment))
			}
		}
	}

	return controllers, nil
}

// makeDeploymentController builds a cluster.Controller from a kubernetes Deployment
func makeDeploymentController(id flux.ResourceID, deployment *apiext.Deployment) cluster.Controller {
	var statusMessage string
	meta, status := deployment.ObjectMeta, deployment.Status
	if status.ObservedGeneration >= meta.Generation {
		// the definition has been updated; now let's see about the replicas
		updated, wanted := status.UpdatedReplicas, *deployment.Spec.Replicas
		if updated == wanted {
			statusMessage = StatusReady
		} else {
			statusMessage = fmt.Sprintf("%d out of %d updated", updated, wanted)
		}
	} else {
		statusMessage = StatusUpdating
	}

	var containers []cluster.Container
	for _, container := range deployment.Spec.Template.Spec.Containers {
		containers = append(containers, cluster.Container{Name: container.Name, Image: container.Image})
	}

	return cluster.Controller{
		ID:         id,
		Status:     statusMessage,
		Containers: cluster.ContainersOrExcuse{Containers: containers},
	}
}

// Sync performs the given actions on resources. Operations are
// asynchronous, but serialised.
func (c *Cluster) Sync(spec cluster.SyncDef) error {
	errc := make(chan error)
	logger := log.NewContext(c.logger).With("method", "Sync")
	c.actionc <- func() {
		errs := cluster.SyncError{}
		for _, action := range spec.Actions {
			logger := log.NewContext(logger).With("resource", action.ResourceID)
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

func (c *Cluster) Export() ([]byte, error) {
	var config bytes.Buffer
	list, err := c.client.Namespaces().List(api.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "getting namespaces")
	}
	for _, ns := range list.Items {
		err := appendYAML(&config, "v1", "Namespace", ns)
		if err != nil {
			return nil, errors.Wrap(err, "marshalling namespace to YAML")
		}

		deployments, err := c.client.Deployments(ns.Name).List(api.ListOptions{})
		if err != nil {
			return nil, errors.Wrap(err, "getting deployments")
		}
		for _, deployment := range deployments.Items {
			if isAddon(&deployment) {
				continue
			}
			err := appendYAML(&config, "extensions/v1beta1", "Deployment", deployment)
			if err != nil {
				return nil, errors.Wrap(err, "marshalling deployment to YAML")
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

// ImagesToFetch is a k8s specific method to get a list of images to update along with their credentials
func (c *Cluster) ImagesToFetch() (imageCreds registry.ImageCreds) {
	imageCreds = make(registry.ImageCreds)

	namespaces, err := c.client.Namespaces().List(api.ListOptions{})
	if err != nil {
		c.logger.Log("err", errors.Wrap(err, "getting namespaces"))
		return
	}

	for _, ns := range namespaces.Items {
		deployments, err := c.client.Deployments(ns.Name).List(api.ListOptions{})
		if err != nil {
			c.logger.Log("err", errors.Wrapf(err, "getting deployments for namespace %s", ns.Name))
			return
		}

		for _, deployment := range deployments.Items {
			creds := registry.NoCredentials()
			for _, imagePullSecret := range deployment.Spec.Template.Spec.ImagePullSecrets {
				secret, err := c.client.Secrets(ns.Name).Get(imagePullSecret.Name)
				if err != nil {
					c.logger.Log("err", errors.Wrapf(err, "getting secret %q from namespace %q", secret.Name, ns.Name))
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
					c.logger.Log("skip", "unknown type", "secret", ns.Name+"/"+secret.Name, "type", secret.Type)
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
			for _, container := range deployment.Spec.Template.Spec.Containers {
				r, err := flux.ParseImageID(container.Image)
				if err != nil {
					c.logger.Log("err", err.Error())
					continue
				}
				imageCreds[r] = creds
			}
		}
	}

	return
}

// --- end cluster.Cluster

// A convenience for getting an minimal object from some bytes.
func definitionObj(bytes []byte) (*apiObject, error) {
	obj := apiObject{bytes: bytes}
	return &obj, yaml.Unmarshal(bytes, &obj)
}
