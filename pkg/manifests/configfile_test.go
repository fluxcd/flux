package manifests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fluxcd/flux/pkg/resource"
)

func TestFailsValidation(t *testing.T) {
	for name, fluxyaml := range map[string]string{
		"empty":                     "",
		"wrong version":             "version: 2",
		"no command/patch/whatever": "version: 1",

		"no generators": `
version: 1
commandUpdated: {}
`,

		"no patchFile": `
version: 1
patchUpdated:
  generators: []
`,
		"more than one": `
version: 1
patchUpdated:
  generators: []
  patchFile: patch.yaml
commandUpdated:
  generators: []
`,

		"duff generator": `
version: 1
patchUpdated:
  generators:
  - not an object
`,

		"patchFile with commandUpdated": `
version: 1
commandUpdated:
  generators: []
  patchFile: "foo.yaml"
`,
	} {
		t.Run(name, func(t *testing.T) {
			var cf ConfigFile
			assert.Error(t, ParseConfigFile([]byte(fluxyaml), &cf))
		})
	}
}

func TestPassesValidation(t *testing.T) {
	for name, fluxyaml := range map[string]string{
		"minimal commandUpdated": `
version: 1
commandUpdated:
  generators: []
`,
		"minimal patchUpdated": `
version: 1
patchUpdated:
  generators: []
  patchFile: foo.yaml
`,

		"minimal files (the only kind)": `
version: 1
scanForFiles: {}
`,
	} {
		t.Run(name, func(t *testing.T) {
			var cf ConfigFile
			assert.NoError(t, ParseConfigFile([]byte(fluxyaml), &cf))
		})
	}
}

const justFilesConfigFile = `
version: 1
scanForFiles: {}
`

func TestJustFileDirective(t *testing.T) {
	var cf ConfigFile
	err := ParseConfigFile([]byte(justFilesConfigFile), &cf)
	assert.NoError(t, err)

	assert.True(t, cf.IsScanForFiles())
}

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
	if err := ParseConfigFile([]byte(patchUpdatedConfigFile), &cf); err != nil {
		t.Fatal(err)
	}
	assert.False(t, cf.IsScanForFiles())
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
	if err := ParseConfigFile([]byte(echoCmdUpdatedConfigFile), &cf); err != nil {
		t.Fatal(err)
	}
	assert.False(t, cf.IsScanForFiles())
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
	err := ParseConfigFile([]byte(echoCmdUpdatedConfigFile), &cf)
	assert.NoError(t, err)
	result := cf.execGenerators(context.Background(), cf.CommandUpdated.Generators)
	assert.Equal(t, 2, len(result), "result: %s", result)
	assert.Equal(t, "g1\n", string(result[0].Stdout))
	assert.Equal(t, "g2\n", string(result[1].Stdout))
}

func TestExecContainerImageUpdaters(t *testing.T) {
	var cf ConfigFile
	err := ParseConfigFile([]byte(echoCmdUpdatedConfigFile), &cf)
	assert.NoError(t, err)
	resourceID := resource.MustParseID("default:deployment/foo")
	result := cf.execContainerImageUpdaters(context.Background(), resourceID, "bar", "repo/image", "latest")
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
	err := ParseConfigFile([]byte(echoCmdUpdatedConfigFile), &cf)
	assert.NoError(t, err)
	resourceID := resource.MustParseID("default:deployment/foo")

	// Test the update/addition of annotations
	annotationValue := "value"
	result := cf.execPolicyUpdaters(context.Background(), resourceID, "key", annotationValue)
	assert.Equal(t, 2, len(result), "result: %s", result)
	assert.Equal(t,
		"ua1 default:deployment/foo default deployment foo key value\n",
		string(result[0].Output))
	assert.Equal(t,
		"ua2 default:deployment/foo default deployment foo key value\n",
		string(result[1].Output))

	// Test the deletion of annotations "
	result = cf.execPolicyUpdaters(context.Background(), resourceID, "key", "")
	assert.Equal(t, 2, len(result), "result: %s", result)
	assert.Equal(t,
		"ua1 default:deployment/foo default deployment foo key delete\n",
		string(result[0].Output))
	assert.Equal(t,
		"ua2 default:deployment/foo default deployment foo key delete\n",
		string(result[1].Output))
}
