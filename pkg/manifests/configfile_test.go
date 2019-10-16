package manifests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"

	"github.com/fluxcd/flux/pkg/resource"
)

const patchUpdatedConfigFile = `---
version: 1
patchUpdated:
  generators: 
    - command: foo
    - command: bar
  patchFile: baz.yaml
`

func TestParsePatchUpdatedConfigFile(t *testing.T) {
	var cf ConfigFile
	if err := yaml.Unmarshal([]byte(patchUpdatedConfigFile), &cf); err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, cf.PatchUpdated)
	assert.Nil(t, cf.CommandUpdated)
	assert.Equal(t, 1, cf.Version)
	assert.Equal(t, 2, len(cf.PatchUpdated.Generators))
	assert.Equal(t, "bar", cf.PatchUpdated.Generators[1].Command)
	assert.Equal(t, "baz.yaml", cf.PatchUpdated.PatchFile)
}

const echoCmdUpdatedConfigFile = `---
version: 1
commandUpdated:
  generators: 
    - command: echo g1
    - command: echo g2
  updaters:
    - containerImage:
        command: echo uci1 $FLUX_WORKLOAD $FLUX_WL_NS $FLUX_WL_KIND $FLUX_WL_NAME $FLUX_CONTAINER $FLUX_IMG $FLUX_TAG
      policy:
        command: echo ua1 $FLUX_WORKLOAD $FLUX_WL_NS $FLUX_WL_KIND $FLUX_WL_NAME $FLUX_POLICY ${FLUX_POLICY_VALUE:-delete}
    - containerImage:
        command: echo uci2 $FLUX_WORKLOAD $FLUX_WL_NS $FLUX_WL_KIND $FLUX_WL_NAME $FLUX_CONTAINER $FLUX_IMG $FLUX_TAG
      policy:
        command: echo ua2 $FLUX_WORKLOAD $FLUX_WL_NS $FLUX_WL_KIND $FLUX_WL_NAME $FLUX_POLICY ${FLUX_POLICY_VALUE:-delete}
`

func TestParseCmdUpdatedConfigFile(t *testing.T) {
	var cf ConfigFile
	if err := yaml.Unmarshal([]byte(echoCmdUpdatedConfigFile), &cf); err != nil {
		t.Fatal(err)
	}
	assert.NotNil(t, cf.CommandUpdated)
	assert.Nil(t, cf.PatchUpdated)
	assert.Equal(t, 1, cf.Version)
	assert.Equal(t, 2, len(cf.CommandUpdated.Generators))
	assert.Equal(t, 2, len(cf.CommandUpdated.Updaters))
	assert.Equal(t,
		"echo uci1 $FLUX_WORKLOAD $FLUX_WL_NS $FLUX_WL_KIND $FLUX_WL_NAME $FLUX_CONTAINER $FLUX_IMG $FLUX_TAG",
		cf.CommandUpdated.Updaters[0].ContainerImage.Command,
	)
	assert.Equal(t,
		"echo ua2 $FLUX_WORKLOAD $FLUX_WL_NS $FLUX_WL_KIND $FLUX_WL_NAME $FLUX_POLICY ${FLUX_POLICY_VALUE:-delete}",
		cf.CommandUpdated.Updaters[1].Policy.Command,
	)
}

func TestExecGenerators(t *testing.T) {
	var cf ConfigFile
	err := yaml.Unmarshal([]byte(echoCmdUpdatedConfigFile), &cf)
	assert.NoError(t, err)
	result := cf.ExecGenerators(context.Background(), cf.CommandUpdated.Generators)
	assert.Equal(t, 2, len(result), "result: %s", result)
	assert.Equal(t, "g1\n", string(result[0].Stdout))
	assert.Equal(t, "g2\n", string(result[1].Stdout))
}

func TestExecContainerImageUpdaters(t *testing.T) {
	var cf ConfigFile
	err := yaml.Unmarshal([]byte(echoCmdUpdatedConfigFile), &cf)
	assert.NoError(t, err)
	resourceID := resource.MustParseID("default:deployment/foo")
	result := cf.ExecContainerImageUpdaters(context.Background(), resourceID, "bar", "repo/image", "latest")
	assert.Equal(t, 2, len(result), "result: %s", result)
	assert.Equal(t,
		"uci1 default:deployment/foo default deployment foo bar repo/image latest\n",
		string(result[0].Output))
	assert.Equal(t,
		"uci2 default:deployment/foo default deployment foo bar repo/image latest\n",
		string(result[1].Output))
}

func TestExecAnnotationUpdaters(t *testing.T) {
	var cf ConfigFile
	err := yaml.Unmarshal([]byte(echoCmdUpdatedConfigFile), &cf)
	assert.NoError(t, err)
	resourceID := resource.MustParseID("default:deployment/foo")

	// Test the update/addition of annotations
	annotationValue := "value"
	result := cf.ExecPolicyUpdaters(context.Background(), resourceID, "key", annotationValue)
	assert.Equal(t, 2, len(result), "result: %s", result)
	assert.Equal(t,
		"ua1 default:deployment/foo default deployment foo key value\n",
		string(result[0].Output))
	assert.Equal(t,
		"ua2 default:deployment/foo default deployment foo key value\n",
		string(result[1].Output))

	// Test the deletion of annotations "
	result = cf.ExecPolicyUpdaters(context.Background(), resourceID, "key", "")
	assert.Equal(t, 2, len(result), "result: %s", result)
	assert.Equal(t,
		"ua1 default:deployment/foo default deployment foo key delete\n",
		string(result[0].Output))
	assert.Equal(t,
		"ua2 default:deployment/foo default deployment foo key delete\n",
		string(result[1].Output))

}
