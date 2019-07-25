package install

import (
	"testing"

	"github.com/instrumenta/kubeval/kubeval"
	"github.com/stretchr/testify/assert"
)

func testFillInInstallTemplates(t *testing.T, params TemplateParameters) {
	manifests, err := FillInInstallTemplates(params)
	assert.NoError(t, err)
	assert.Len(t, manifests, 5)
	for fileName, contents := range manifests {
		validationResults, err := kubeval.Validate(contents, fileName)
		assert.NoError(t, err)
		assert.Len(t, validationResults, 1)
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

func TestFillInInstallTemplatesAllParameters(t *testing.T) {
	testFillInInstallTemplates(t, TemplateParameters{
		GitURL:             "git@github.com:fluxcd/flux-get-started",
		GitBranch:          "branch",
		GitPaths:           []string{"dir1", "dir2"},
		GitLabel:           "label",
		GitUser:            "User",
		GitEmail:           "this.is@anemail.com",
		Namespace:          "flux",
		AdditionalFluxArgs: []string{"arg1=foo", "arg2=bar"},
	})

}

func TestFillInInstallTemplatesMissingValues(t *testing.T) {
	testFillInInstallTemplates(t, TemplateParameters{
		GitURL:    "git@github.com:fluxcd/flux-get-started",
		GitBranch: "branch",
		GitPaths:  []string{},
		GitLabel:  "label",
	})
}
