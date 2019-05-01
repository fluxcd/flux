package resourcestore

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster/kubernetes"
	"github.com/weaveworks/flux/git/gittest"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/policy"
)

func setup(t *testing.T, configFileBody string) (*configFileManager, func()) {
	mockManifests := kubernetes.NewManifests(kubernetes.ConstNamespacer("default"), log.NewLogfmtLogger(os.Stdout))
	var cf ConfigFile
	err := yaml.Unmarshal([]byte(configFileBody), &cf)
	checkout, cleanup := gittest.Checkout(t)
	cf.WorkingDir = checkout.Dir()
	cf.Path = filepath.Join(checkout.Dir(), ConfigFilename)
	assert.NoError(t, err)
	cfm := &configFileManager{
		ctx:                  context.Background(),
		checkout:             checkout,
		configFile:           &cf,
		publicConfigFilePath: ConfigFilename,
		policyTranslator:     &kubernetes.PolicyTranslator{},
		manifests:            mockManifests,
	}
	return cfm, cleanup
}

const commandUpdatedEchoConfigFile = `---
version: 1
commandUpdated:
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

func TestCommandUpdatedConfigFileManager(t *testing.T) {
	cfm, cleanup := setup(t, commandUpdatedEchoConfigFile)
	defer cleanup()
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

const patchUpdatedEchoConfigFile = `---
version: 1
patchUpdated:
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
               image: quay.io/weaveworks/helloworld:master-a000001
       ---
       apiVersion: v1
       kind: Namespace
       metadata:
         name: demo"
  patchFile: patchfile.yaml
`

func TestPatchUpdatedConfigFileManager(t *testing.T) {
	cfm, cleanup := setup(t, patchUpdatedEchoConfigFile)
	defer cleanup()
	resources, err := cfm.GetAllResources()
	assert.NoError(t, err)
	assert.Equal(t, 2, len(resources))
	// (wrongly) assume the resources are returned in input order to simplify the test
	var deployment updatableResource
	for _, res := range resources {
		if res.GetResource().ResourceID() == flux.MustParseResourceID("default:deployment/helloworld") {
			deployment = res
		}
	}
	assert.NotNil(t, deployment)
	ref, err := image.ParseRef("repo/image:tag")
	assert.NoError(t, err)
	err = deployment.SetWorkloadContainerImage("greeter", ref)
	assert.NoError(t, err)
	_, err = deployment.UpdateWorkloadPolicies(policy.Update{
		Add: policy.Set{policy.TagPrefix("greeter"): "glob:master-*"},
	})
	expectedPatch := `---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  annotations:
    flux.weave.works/tag.greeter: glob:master-*
  name: helloworld
spec:
  template:
    spec:
      $setElementOrder/containers:
      - name: greeter
      containers:
      - image: repo/image:tag
        name: greeter
`
	patchFilePath := filepath.Join(cfm.checkout.Dir(), "patchfile.yaml")
	patch, err := ioutil.ReadFile(patchFilePath)
	assert.NoError(t, err)
	assert.Equal(t, expectedPatch, string(patch))
}
