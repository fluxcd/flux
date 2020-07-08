package kubernetes

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/registry"
	"github.com/fluxcd/flux/pkg/resource"
)

func mergeCredentials(logger *zap.Logger,
	includeImage func(imageName string) bool,
	client ExtendedClient,
	namespace string, podTemplate apiv1.PodTemplateSpec,
	imageCreds registry.ImageCreds,
	imagePullSecretCache map[string]registry.Credentials) {
	var images []image.Name
	for _, container := range podTemplate.Spec.InitContainers {
		r, err := image.ParseRef(container.Image)
		if err != nil {
			logger.Error("error parsing container image", zap.String("image", container.Image), zap.NamedError("err", err))
			continue
		}
		if includeImage(r.CanonicalName().Name.String()) {
			images = append(images, r.Name)
		}
	}

	for _, container := range podTemplate.Spec.Containers {
		r, err := image.ParseRef(container.Image)
		if err != nil {
			logger.Error("error parsing container image", zap.String("image", container.Image), zap.NamedError("err", err))
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
		namespacedSecretName := fmt.Sprintf("%s/%s", namespace, name)
		if seen, ok := imagePullSecretCache[namespacedSecretName]; ok {
			creds.Merge(seen)
			continue
		}

		secret, err := client.CoreV1().Secrets(namespace).Get(name, meta_v1.GetOptions{})
		if err != nil {
			logger.Error(
				"error retrieving secret",
				zap.String("name", name),
				zap.String("namespace", namespace),
				zap.NamedError("err", err),
			)
			imagePullSecretCache[namespacedSecretName] = registry.NoCredentials()
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
			logger.Info(
				"unknown secret type",
				zap.String("skip", "unknown type"),
				zap.String("secret", namespace+"/"+secret.Name),
				zap.String("type", string(secret.Type)),
			)
			imagePullSecretCache[namespacedSecretName] = registry.NoCredentials()
			continue
		}

		if !ok {
			logger.Error(
				"error retrieving pod secret",
				zap.String("name", secret.Name),
				zap.String("namespace", namespace),
				zap.NamedError("err", err),
			)
			imagePullSecretCache[namespacedSecretName] = registry.NoCredentials()
			continue
		}

		// Parse secret
		crd, err := registry.ParseCredentials(fmt.Sprintf("%s:secret/%s", namespace, name), decoded)
		if err != nil {
			logger.Error(
				"error parsing credentials",
				zap.String("secret_name", secret.Name),
				zap.String("namespace", namespace),
				zap.NamedError("err", err),
			)
			imagePullSecretCache[namespacedSecretName] = registry.NoCredentials()
			continue
		}
		imagePullSecretCache[namespacedSecretName] = crd

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

	namespaces, err := c.getAllowedAndExistingNamespaces(ctx)
	if err != nil {
		c.logger.Error(
			"error getting namespaces",
			zap.NamedError("err", err),
		)
		return allImageCreds
	}

	for _, ns := range namespaces {
		imagePullSecretCache := make(map[string]registry.Credentials) // indexed by the namespace/name of pullImageSecrets
		for kind, resourceKind := range resourceKinds {
			workloads, err := resourceKind.getWorkloads(ctx, c, ns)
			if err != nil {
				if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
					// Skip unsupported or forbidden resource kinds
					continue
				}
				c.logger.Error(
					"error getting resource",
					zap.String("kind", kind),
					zap.String("namespace", ns),
					zap.NamedError("err", err),
				)
			}

			imageCreds := make(registry.ImageCreds)
			for _, workload := range workloads {
				logger := c.logger.With(zap.Any("resource", resource.MakeID(workload.GetNamespace(), kind, workload.GetName())))
				mergeCredentials(logger, c.imageIncluder.IsIncluded, c.client, workload.GetNamespace(), workload.podTemplate, imageCreds, imagePullSecretCache)
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
