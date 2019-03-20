package resourcestore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"

	"github.com/weaveworks/flux"
)

const echoConfigFile = `---
version: 1
generators: 
  - command: echo g1
  - command: echo g2
updaters:
  - containerImage:
      command: echo uci1 $FLUX_WORKLOAD $FLUX_WL_NS $FLUX_WL_KIND $FLUX_WL_NAME $FLUX_CONTAINER $FLUX_IMG $FLUX_TAG
    annotation:
      command: echo ua1 $FLUX_WORKLOAD $FLUX_WL_NS $FLUX_WL_KIND $FLUX_WL_NAME $FLUX_ANNOTATION_KEY ${FLUX_ANNOTATION_VALUE:-delete}
  - containerImage:
      command: echo uci2 $FLUX_WORKLOAD $FLUX_WL_NS $FLUX_WL_KIND $FLUX_WL_NAME $FLUX_CONTAINER $FLUX_IMG $FLUX_TAG
    annotation:
      command: echo ua2 $FLUX_WORKLOAD $FLUX_WL_NS $FLUX_WL_KIND $FLUX_WL_NAME $FLUX_ANNOTATION_KEY ${FLUX_ANNOTATION_VALUE:-delete}
`

func TestParseConfigFile(t *testing.T) {
	var cf ConfigFile
	if err := yaml.Unmarshal([]byte(echoConfigFile), &cf); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "1", cf.Version)
	assert.Equal(t, 2, len(cf.Generators))
	assert.Equal(t, 2, len(cf.Updaters))
	assert.Equal(t,
		"echo uci1 $FLUX_WORKLOAD $FLUX_WL_NS $FLUX_WL_KIND $FLUX_WL_NAME $FLUX_CONTAINER $FLUX_IMG $FLUX_TAG",
		cf.Updaters[0].ContainerImage.Command,
	)
	assert.Equal(t,
		"echo ua2 $FLUX_WORKLOAD $FLUX_WL_NS $FLUX_WL_KIND $FLUX_WL_NAME $FLUX_ANNOTATION_KEY ${FLUX_ANNOTATION_VALUE:-delete}",
		cf.Updaters[1].Annotation.Command,
	)
}

func TestExecGenerators(t *testing.T) {
	var cf ConfigFile
	err := yaml.Unmarshal([]byte(echoConfigFile), &cf)
	assert.NoError(t, err)
	result := cf.ExecGenerators(context.Background())
	assert.Equal(t, 2, len(result), "result: %s", result)
	assert.Equal(t, "g1\n", string(result[0].Stdout))
	assert.Equal(t, "g2\n", string(result[1].Stdout))
}

func TestExecContainerImageUpdaters(t *testing.T) {
	var cf ConfigFile
	err := yaml.Unmarshal([]byte(echoConfigFile), &cf)
	assert.NoError(t, err)
	resourceID := flux.MustParseResourceID("default:deployment/foo")
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
	err := yaml.Unmarshal([]byte(echoConfigFile), &cf)
	assert.NoError(t, err)
	resourceID := flux.MustParseResourceID("default:deployment/foo")

	// Test the update/addition of annotations
	annotationValue := "value"
	result := cf.ExecAnnotationUpdaters(context.Background(), resourceID, "key", &annotationValue)
	assert.Equal(t, 2, len(result), "result: %s", result)
	assert.Equal(t,
		"ua1 default:deployment/foo default deployment foo key value\n",
		string(result[0].Output))
	assert.Equal(t,
		"ua2 default:deployment/foo default deployment foo key value\n",
		string(result[1].Output))

	// Test the deletion of annotations "
	result = cf.ExecAnnotationUpdaters(context.Background(), resourceID, "key", nil)
	assert.Equal(t, 2, len(result), "result: %s", result)
	assert.Equal(t,
		"ua1 default:deployment/foo default deployment foo key delete\n",
		string(result[0].Output))
	assert.Equal(t,
		"ua2 default:deployment/foo default deployment foo key delete\n",
		string(result[1].Output))

}
