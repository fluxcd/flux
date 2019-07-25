package install

import (
	"io/ioutil"
	"testing"

	"github.com/instrumenta/kubeval/kubeval"
	"github.com/stretchr/testify/assert"
)

func TestFillInInstallTemplates(t *testing.T) {
	params := TemplateParameters{
		GitURL:             "git@github.com:fluxcd/flux-get-started",
		GitBranch:          "branch",
		GitPaths:           []string{"dir1", "dir2"},
		GitLabel:           "label",
		GitUser:            "User",
		GitEmail:           "this.is@anemail.com",
		Namespace:          "flux",
		AdditionalFluxArgs: []string{"arg1=foo", "arg2=bar"},
	}
	reader, err := FillInInstallTemplates(params)
	assert.NoError(t, err)
	output, err := ioutil.ReadAll(reader)
	assert.NoError(t, err)
	validationResults, err := kubeval.Validate(output, "output")
	assert.NoError(t, err)
	assert.Len(t, validationResults, 7)
	for _, result := range validationResults {
		if len(result.Errors) > 0 {
			t.Errorf("found problems with resource resource %s: %s", result.Kind, result.Errors)
		}
	}
}
