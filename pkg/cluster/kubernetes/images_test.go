package kubernetes

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/ryanuber/go-glob"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/registry"
)

func noopLog(e ...interface{}) error {
	fmt.Println(e...)
	return nil
}

func makeImagePullSecret(ns, name, host string) *apiv1.Secret {
	imagePullSecret := apiv1.Secret{Type: apiv1.SecretTypeDockerConfigJson}
	imagePullSecret.Name = name
	imagePullSecret.Namespace = ns
	imagePullSecret.Data = map[string][]byte{
		apiv1.DockerConfigJsonKey: []byte(`
{
  "auths": {
    "` + host + `": {
      "auth": "` + base64.StdEncoding.EncodeToString([]byte("user:passwd")) + `"
      }
    }
}`),
	}
	return &imagePullSecret
}

func makeServiceAccount(ns, name string, imagePullSecretNames []string) *apiv1.ServiceAccount {
	sa := apiv1.ServiceAccount{}
	sa.Namespace = ns
	sa.Name = name
	for _, ips := range imagePullSecretNames {
		sa.ImagePullSecrets = append(sa.ImagePullSecrets, apiv1.LocalObjectReference{Name: ips})
	}
	return &sa
}

func TestMergeCredentials(t *testing.T) {
	ns, secretName1, secretName2 := "foo-ns", "secret-creds", "secret-sa-creds"
	saName := "service-account"
	ref, _ := image.ParseRef("foo/bar:tag")
	spec := apiv1.PodTemplateSpec{
		Spec: apiv1.PodSpec{
			ServiceAccountName: saName,
			ImagePullSecrets: []apiv1.LocalObjectReference{
				{Name: secretName1},
			},
			Containers: []apiv1.Container{
				{Name: "container1", Image: ref.String()},
			},
		},
	}

	clientset := fake.NewSimpleClientset(
		makeServiceAccount(ns, saName, []string{secretName2}),
		makeImagePullSecret(ns, secretName1, "docker.io"),
		makeImagePullSecret(ns, secretName2, "quay.io"))
	client := ExtendedClient{coreClient: clientset}

	creds := registry.ImageCreds{}

	mergeCredentials(noopLog, func(imageName string) bool { return true },
		client, ns, spec, creds, make(map[string]registry.Credentials))

	// check that we accumulated some credentials
	assert.Contains(t, creds, ref.Name)
	c := creds[ref.Name]
	hosts := c.Hosts()
	assert.ElementsMatch(t, []string{"docker.io", "quay.io"}, hosts)
}

func TestMergeCredentials_SameSecretSameNameDifferentNamespace(t *testing.T) {
	ns1, ns2, secretName := "foo-ns", "bar-ns", "pull-secretname"
	saName := "service-account"
	ref, _ := image.ParseRef("foo/bar:tag")
	spec := apiv1.PodTemplateSpec{
		Spec: apiv1.PodSpec{
			ServiceAccountName: saName,
			ImagePullSecrets: []apiv1.LocalObjectReference{
				{Name: secretName},
			},
			Containers: []apiv1.Container{
				{Name: "container1", Image: ref.String()},
			},
		},
	}

	clientset := fake.NewSimpleClientset(
		makeServiceAccount(ns1, saName, []string{secretName}),
		makeServiceAccount(ns2, saName, []string{secretName}),
		makeImagePullSecret(ns1, secretName, "docker.io"),
		makeImagePullSecret(ns2, secretName, "quay.io"))
	client := ExtendedClient{coreClient: clientset}

	creds := registry.ImageCreds{}

	pullImageSecretCache := make(map[string]registry.Credentials)
	mergeCredentials(noopLog, func(imageName string) bool { return true },
		client, ns1, spec, creds, pullImageSecretCache)
	mergeCredentials(noopLog, func(imageName string) bool { return true },
		client, ns2, spec, creds, pullImageSecretCache)
	// check that we accumulated some credentials
	assert.Contains(t, creds, ref.Name)
	c := creds[ref.Name]
	hosts := c.Hosts()
	// Make sure we get the host from the second secret
	assert.ElementsMatch(t, []string{"quay.io"}, hosts)
}

func TestMergeCredentials_ImageExclusion(t *testing.T) {
	creds := registry.ImageCreds{}
	gcrImage, _ := image.ParseRef("gcr.io/foo/bar:tag")
	k8sImage, _ := image.ParseRef("k8s.gcr.io/foo/bar:tag")
	testImage, _ := image.ParseRef("docker.io/test/bar:tag")

	spec := apiv1.PodTemplateSpec{
		Spec: apiv1.PodSpec{
			InitContainers: []apiv1.Container{
				{Name: "container1", Image: testImage.String()},
			},
			Containers: []apiv1.Container{
				{Name: "container1", Image: k8sImage.String()},
				{Name: "container2", Image: gcrImage.String()},
			},
		},
	}

	clientset := fake.NewSimpleClientset()
	client := ExtendedClient{coreClient: clientset}

	var includeImage = func(imageName string) bool {
		for _, exp := range []string{"k8s.gcr.io/*", "*test*"} {
			if glob.Glob(exp, imageName) {
				return false
			}
		}
		return true
	}

	mergeCredentials(noopLog, includeImage, client, "default", spec, creds,
		make(map[string]registry.Credentials))

	// check test image has been excluded
	assert.NotContains(t, creds, testImage.Name)

	// check k8s.gcr.io image has been excluded
	assert.NotContains(t, creds, k8sImage.Name)

	// check gcr.io image exists
	assert.Contains(t, creds, gcrImage.Name)
}
