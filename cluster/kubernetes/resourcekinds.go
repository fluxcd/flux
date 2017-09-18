package kubernetes

import (
	"bytes"
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/client-go/1.5/pkg/api"
	apiext "k8s.io/client-go/1.5/pkg/apis/extensions/v1beta1"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/registry"
)

// MakeController builds a cluster.Controller for a specific resourceID.
func MakeController(c *Cluster, id flux.ResourceID) (*cluster.Controller, error) {
	_, kind, _ := id.Components()

	resourceKind := resourceKinds[kind]
	if resourceKind == nil {
		return nil, fmt.Errorf("Unsupported kind %v", kind)
	}

	return resourceKind.makeController(c, id)
}

// MakeAllControllers builds a cluster.Controller for all supported kinds of resource
// in the specified namespace.
func MakeAllControllers(c *Cluster, namespace string) ([]cluster.Controller, error) {
	var allControllers []cluster.Controller
	for _, resourceKind := range resourceKinds {
		controllers, err := resourceKind.makeAllControllers(c, namespace)
		if err != nil {
			return nil, err
		}
		allControllers = append(allControllers, controllers...)
	}
	return allControllers, nil
}

// MakeAllImageCreds returns a credentials map for all images specified by every kind of
// supported resource.
func MakeAllImageCreds(c *Cluster) registry.ImageCreds {
	allImageCreds := make(registry.ImageCreds)

	namespaces, err := c.client.Namespaces().List(api.ListOptions{})
	if err != nil {
		c.logger.Log("err", errors.Wrap(err, "getting namespaces"))
		return allImageCreds
	}

	for _, ns := range namespaces.Items {
		for _, resourceKind := range resourceKinds {
			imageCreds := resourceKind.makeAllImageCreds(c, ns.Name)

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

func AppendYAML(c *Cluster, namespace string, buffer *bytes.Buffer) error {
	for _, resourceKind := range resourceKinds {
		if err := resourceKind.appendYAML(c, namespace, buffer); err != nil {
			return err
		}
	}
	return nil
}

/////////////////////////////////////////////////////////////////////////////
// Kind registry

type resourceKind interface {
	makeController(c *Cluster, id flux.ResourceID) (*cluster.Controller, error)
	makeAllControllers(c *Cluster, namespace string) ([]cluster.Controller, error)
	makeAllImageCreds(c *Cluster, namespace string) registry.ImageCreds
	appendYAML(c *Cluster, namespace string, buffer *bytes.Buffer) error
}

var (
	resourceKinds = make(map[string]resourceKind)
)

func init() {
	resourceKinds["deployment"] = &deploymentKind{}
}

/////////////////////////////////////////////////////////////////////////////
// extensions/v1beta1 Deployment

type deploymentKind struct{}

func (*deploymentKind) makeController(c *Cluster, id flux.ResourceID) (*cluster.Controller, error) {
	ns, _, name := id.Components()

	deployment, err := c.client.Deployments(ns).Get(name)
	if err != nil {
		return nil, errors.Wrapf(err, "fetching deployment %s for namespace %S", name, ns)
	}
	if isAddon(deployment) {
		return nil, nil
	}
	return makeDeploymentController(id, deployment), nil
}

func (*deploymentKind) makeAllControllers(c *Cluster, namespace string) ([]cluster.Controller, error) {
	var controllers []cluster.Controller
	deployments, err := c.client.Deployments(namespace).List(api.ListOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "getting deployments for namespace %s", namespace)
	}

	for _, deployment := range deployments.Items {
		if !isAddon(&deployment) {
			id := flux.MakeResourceID(namespace, "deployment", deployment.Name)
			controllers = append(controllers, *makeDeploymentController(id, &deployment))
		}
	}

	return controllers, nil
}

func (*deploymentKind) makeAllImageCreds(c *Cluster, namespace string) registry.ImageCreds {
	imageCreds := make(registry.ImageCreds)

	deployments, err := c.client.Deployments(namespace).List(api.ListOptions{})
	if err != nil {
		c.logger.Log("err", errors.Wrapf(err, "getting deployments for namespace %s", namespace))
		return imageCreds
	}

	for _, deployment := range deployments.Items {
		creds := registry.NoCredentials()
		for _, imagePullSecret := range deployment.Spec.Template.Spec.ImagePullSecrets {
			secret, err := c.client.Secrets(namespace).Get(imagePullSecret.Name)
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
		for _, container := range deployment.Spec.Template.Spec.Containers {
			r, err := flux.ParseImageID(container.Image)
			if err != nil {
				c.logger.Log("err", err.Error())
				continue
			}
			imageCreds[r] = creds
		}
	}

	return imageCreds
}

func (*deploymentKind) appendYAML(c *Cluster, namespace string, buffer *bytes.Buffer) error {
	deployments, err := c.client.Deployments(namespace).List(api.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "getting deployments")
	}
	for _, deployment := range deployments.Items {
		if isAddon(&deployment) {
			continue
		}
		if err := appendYAML(buffer, "extensions/v1beta1", "Deployment", deployment); err != nil {
			return errors.Wrap(err, "marshalling deployment to YAML")
		}
	}
	return nil
}

// makeDeploymentController builds a cluster.Controller from a kubernetes Deployment
func makeDeploymentController(id flux.ResourceID, deployment *apiext.Deployment) *cluster.Controller {
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

	return &cluster.Controller{
		ID:         id,
		Status:     statusMessage,
		Containers: cluster.ContainersOrExcuse{Containers: containers},
	}
}

/////////////////////////////////////////////////////////////////////////////
//
