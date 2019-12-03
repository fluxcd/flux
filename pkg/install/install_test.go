package install

import (
	"testing"

	"github.com/instrumenta/kubeval/kubeval"
	"github.com/stretchr/testify/assert"
)

func testFillInTemplates(t *testing.T, expectedManifestCount int, params TemplateParameters) {
	manifests, err := FillInTemplates(params)
	assert.NoError(t, err)
	assert.Len(t, manifests, expectedManifestCount)
	for fileName, contents := range manifests {
		validationResults, err := kubeval.Validate(contents, config)
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
