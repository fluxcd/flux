package kubernetes

import (
	"context"
	"fmt"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"github.com/ryanuber/go-glob"
	apiv1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/registry"
	"github.com/fluxcd/flux/pkg/resource"
)

func mergeCredentials(log func(...interface{}) error,
	includeImage func(imageName string) bool,
	client ExtendedClient,
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
	ctx := context.Background()

	seenCreds := make(map[string]registry.Credentials)
	workloads, err := c.allWorkfloads(ctx, "")
	if err != nil {
		c.logger.Log("err", errors.Wrap(err, "getting namespaces"))
		return allImageCreds
	}

	imageCreds := make(registry.ImageCreds)
	for id, workload := range workloads {
		ns, kind, _ := id.Components()
		logger := log.With(c.logger, "resource", resource.MakeID(ns, kind, workload.GetName()))
		mergeCredentials(logger.Log, c.includeImage, c.client, ns, workload.podTemplate, imageCreds, seenCreds)
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
