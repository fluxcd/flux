package resourcestore

import (
	"context"
	"os"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster/kubernetes"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/policy"
)

const deploymentConfigFile = `---
version: 1
generators: 
  - command: |
     echo "apiVersion: extensions/v1beta1
     kind: Deployment
     metadata:
       name: helloworld
     spec:
       template:
         metadata:
           labels:
            name: helloworld
         spec:
           containers:
           - name: greeter
             image: quay.io/weaveworks/helloworld:master-a000001"
updaters:
  - containerImage:
      command: echo uci $FLUX_WORKLOAD
    annotation:
      command: echo ua $FLUX_WORKLOAD
`

func TestConfigFileManager(t *testing.T) {
	mockManifests := kubernetes.NewManifests(kubernetes.ConstNamespacer("default"), log.NewLogfmtLogger(os.Stdout))
	var cf ConfigFile
	err := yaml.Unmarshal([]byte(deploymentConfigFile), &cf)
	cf.WorkingDir = os.TempDir()
	assert.NoError(t, err)
	cfm := &configFileManager{
		ctx:              context.Background(),
		checkoutDir:      os.TempDir(),
		configFile:       &cf,
		policyTranslator: &kubernetes.PolicyTranslator{},
		manifests:        mockManifests,
	}
	resources, err := cfm.GetAllResources()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(resources))
	res := resources[0]
	assert.Equal(t, flux.MustParseResourceID("default:deployment/helloworld"), res.GetResource().ResourceID())
	ref, err := image.ParseRef("repo/image:tag")
	assert.NoError(t, err)
	err = res.SetWorkloadContainerImage("greeter", ref)
	assert.NoError(t, err)
	_, err = res.UpdateWorkloadPolicies(policy.Update{
		Add: policy.Set{policy.TagPrefix("greeter"): "glob:master-*"},
	})
	assert.NoError(t, err)
}
