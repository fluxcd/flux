package kubernetes

import (
	"fmt"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"github.com/ryanuber/go-glob"
	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/registry"
)

func mergeCredentials(log func(...interface{}) error,
	includeImage func(imageName string) bool,
	client extendedClient,
	namespace string, podTemplate apiv1.PodTemplateSpec,
	imageCreds registry.ImageCreds,
	seenCreds map[string]registry.Credentials) {
	var images []image.Name
	for _, container := range podTemplate.Spec.InitContainers {
		r, err := image.ParseRef(container.Image)
		if err != nil {
			log("err", err.Error())
			continue
		}
		if includeImage(r.CanonicalName().Name.String()) {
			images = append(images, r.Name)
		}
	}

	for _, container := range podTemplate.Spec.Containers {
		r, err := image.ParseRef(container.Image)
		if err != nil {
			log("err", err.Error())
			continue
		}
		if includeImage(r.CanonicalName().Name.String()) {
			images = append(images, r.Name)
		}
	}

	if len(images) < 1 {
		return
	}

	creds := registry.NoCredentials()
	var imagePullSecrets []string
	saName := podTemplate.Spec.ServiceAccountName
	if saName == "" {
		saName = "default"
	}

	sa, err := client.CoreV1().ServiceAccounts(namespace).Get(saName, meta_v1.GetOptions{})
	if err == nil {
		for _, ips := range sa.ImagePullSecrets {
			imagePullSecrets = append(imagePullSecrets, ips.Name)
		}
	}

	for _, imagePullSecret := range podTemplate.Spec.ImagePullSecrets {
		imagePullSecrets = append(imagePullSecrets, imagePullSecret.Name)
	}

	for _, name := range imagePullSecrets {
		if seen, ok := seenCreds[name]; ok {
			creds.Merge(seen)
			continue
		}

		secret, err := client.CoreV1().Secrets(namespace).Get(name, meta_v1.GetOptions{})
		if err != nil {
			log("err", errors.Wrapf(err, "getting secret %q from namespace %q", name, namespace))
			seenCreds[name] = registry.NoCredentials()
			continue
		}

		var decoded []byte
		var ok bool
		// These differ in format; but, ParseCredentials will
		// handle either.
		switch apiv1.SecretType(secret.Type) {
		case apiv1.SecretTypeDockercfg:
			decoded, ok = secret.Data[apiv1.DockerConfigKey]
		case apiv1.SecretTypeDockerConfigJson:
			decoded, ok = secret.Data[apiv1.DockerConfigJsonKey]
		default:
			log("skip", "unknown type", "secret", namespace+"/"+secret.Name, "type", secret.Type)
			seenCreds[name] = registry.NoCredentials()
			continue
		}

		if !ok {
			log("err", errors.Wrapf(err, "retrieving pod secret %q", secret.Name))
			seenCreds[name] = registry.NoCredentials()
			continue
		}

		// Parse secret
		crd, err := registry.ParseCredentials(fmt.Sprintf("%s:secret/%s", namespace, name), decoded)
		if err != nil {
			log("err", err.Error())
			seenCreds[name] = registry.NoCredentials()
			continue
		}
		seenCreds[name] = crd

		// Merge into the credentials for this PodSpec
		creds.Merge(crd)
	}

	// Now create the service and attach the credentials
	for _, image := range images {
		imageCreds[image] = creds
	}
}

// ImagesToFetch is a k8s specific method to get a list of images to update along with their credentials
func (c *Cluster) ImagesToFetch() registry.ImageCreds {
	allImageCreds := make(registry.ImageCreds)

	namespaces, err := c.getAllowedNamespaces()
	if err != nil {
		c.logger.Log("err", errors.Wrap(err, "getting namespaces"))
		return allImageCreds
	}

	for _, ns := range namespaces {
		seenCreds := make(map[string]registry.Credentials)
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
				logger := log.With(c.logger, "resource", flux.MakeResourceID(ns.Name, kind, podController.name))
				mergeCredentials(logger.Log, c.includeImage, c.client, ns.Name, podController.podTemplate, imageCreds, seenCreds)
			}

			// Merge creds
			for imageID, creds := range imageCreds {
				existingCreds, ok := allImageCreds[imageID]
				if ok {
					mergedCreds := registry.NoCredentials()
					mergedCreds.Merge(existingCreds)
					mergedCreds.Merge(creds)
					allImageCreds[imageID] = mergedCreds
				} else {
					allImageCreds[imageID] = creds
				}
			}
		}
	}

	return allImageCreds
}

func (c *Cluster) includeImage(imageName string) bool {
	for _, exp := range c.imageExcludeList {
		if glob.Glob(exp, imageName) {
			return false
		}
	}
	return true
}
