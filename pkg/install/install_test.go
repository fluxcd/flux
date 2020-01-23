package install

import (
	"bytes"
	"strings"
	"testing"

	"github.com/instrumenta/kubeval/kubeval"
	"github.com/stretchr/testify/assert"

	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func testFillInTemplates(t *testing.T, expectedNoManifestst int, params TemplateParameters) map[string][]byte {
	manifests, err := FillInTemplates(params)
	assert.NoError(t, err)
	assert.Len(t, manifests, expectedNoManifestst)

	for fileName, contents := range manifests {
		validationResults, err := kubeval.Validate(contents)
		assert.NoError(t, err)
		for _, result := range validationResults {
			if len(result.Errors) > 0 {
				t.Errorf("found problems with manifest %s (Kind %s):\ncontent:\n%s\nerrors: %s",
					fileName,
					result.Kind,
					string(contents),
					result.Errors)
			}
		}
	}

	return manifests
}

func testExtractDeployment(t *testing.T, data []byte) *v1.Deployment {
	manifest := string(data)
	manifest = strings.Replace(manifest, "---\n", "", -1)

	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewBuffer([]byte(manifest)), 4096)
	deployment := new(v1.Deployment)
	err := decoder.Decode(deployment)
	if err != nil {
		t.Errorf("issue decoding memcache-dep.yaml into a deployment: %#v", err)
	}

	return deployment
}

func TestFillInTemplatesAllParameters(t *testing.T) {
	testFillInTemplates(t, 5, TemplateParameters{
		GitURL:             "git@github.com:fluxcd/flux-get-started",
		GitBranch:          "branch",
		GitPaths:           []string{"dir1", "dir2"},
		GitLabel:           "label",
		GitUser:            "User",
		GitEmail:           "this.is@anemail.com",
		Namespace:          "flux",
		GitReadOnly:        false,
		ManifestGeneration: true,
		AdditionalFluxArgs: []string{"arg1=foo", "arg2=bar"},
		RegistryScanning:   true,
		AddSecurityContext: true,
	})
}

func TestFillInTemplatesMissingValues(t *testing.T) {
	testFillInTemplates(t, 5, TemplateParameters{
		GitURL:           "git@github.com:fluxcd/flux-get-started",
		GitBranch:        "branch",
		GitPaths:         []string{},
		GitLabel:         "label",
		RegistryScanning: true,
	})
}

func TestFillInTemplatesNoMemcached(t *testing.T) {
	testFillInTemplates(t, 3, TemplateParameters{
		GitURL:           "git@github.com:fluxcd/flux-get-started",
		GitBranch:        "branch",
		GitPaths:         []string{},
		GitLabel:         "label",
		RegistryScanning: false,
	})
	testFillInTemplates(t, 3, TemplateParameters{
		GitURL:      "git@github.com:fluxcd/flux-get-started",
		GitBranch:   "branch",
		GitPaths:    []string{},
		GitLabel:    "label",
		GitReadOnly: false,
	})
}

func TestTestFillInTemplatesAddSecurityContext(t *testing.T) {
	params := TemplateParameters{
		GitURL:             "git@github.com:fluxcd/flux-get-started",
		GitBranch:          "branch",
		GitPaths:           []string{"dir1", "dir2"},
		GitLabel:           "label",
		GitUser:            "User",
		GitEmail:           "this.is@anemail.com",
		Namespace:          "flux",
		GitReadOnly:        false,
		ManifestGeneration: true,
		AdditionalFluxArgs: []string{"arg1=foo", "arg2=bar"},
		RegistryScanning:   true,
		AddSecurityContext: true,
	}

	manifests := testFillInTemplates(t, 5, params)

	memDeploy := manifests["memcache-dep.yaml"]
	deployment := testExtractDeployment(t, memDeploy)

	container := deployment.Spec.Template.Spec.Containers[0]
	if container.SecurityContext == nil {
		t.Error("security context not found when there should be one")
	}
}

func TestFillInTemplatesNoSecurityContext(t *testing.T) {
	params := TemplateParameters{
		GitURL:             "git@github.com:fluxcd/flux-get-started",
		GitBranch:          "branch",
		GitPaths:           []string{"dir1", "dir2"},
		GitLabel:           "label",
		GitUser:            "User",
		GitEmail:           "this.is@anemail.com",
		Namespace:          "flux",
		GitReadOnly:        false,
		ManifestGeneration: true,
		AdditionalFluxArgs: []string{"arg1=foo", "arg2=bar"},
		RegistryScanning:   true,
		AddSecurityContext: false,
	}

	manifests := testFillInTemplates(t, 5, params)
	memDeploy := manifests["memcache-dep.yaml"]

	deployment := testExtractDeployment(t, memDeploy)

	container := deployment.Spec.Template.Spec.Containers[0]
	if container.SecurityContext != nil {
		t.Errorf("security context found when there should be none: %#v", container.SecurityContext)
	}
}
