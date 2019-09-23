package manifests

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"

	"github.com/fluxcd/flux/pkg/cluster/kubernetes"
	"github.com/fluxcd/flux/pkg/cluster/kubernetes/testfiles"
	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/policy"
	"github.com/fluxcd/flux/pkg/resource"
)

func TestFindConfigFilePaths(t *testing.T) {
	baseDir, clean := testfiles.TempDir(t)
	defer clean()
	targetPath := filepath.Join(baseDir, "one/two/three")

	// create file structure
	err := os.MkdirAll(targetPath, 0777)
	assert.NoError(t, err)

	// no file should be found in the bottom dir before adding any
	_, _, err = findConfigFilePaths(baseDir, targetPath)
	assert.Equal(t, configFileNotFoundErr, err)

	// a file should be found in the base directory when added
	baseConfigFilePath := filepath.Join(baseDir, ConfigFilename)
	f, err := os.Create(baseConfigFilePath)
	assert.NoError(t, err)
	f.Close()
	configFilePath, workingDir, err := findConfigFilePaths(baseDir, targetPath)
	assert.NoError(t, err)
	assert.Equal(t, baseConfigFilePath, configFilePath)
	assert.Equal(t, targetPath, workingDir)

	// a file should be found in the target directory when added,
	// and preferred over any files in parent directories
	targetConfigFilePath := filepath.Join(targetPath, ConfigFilename)
	f, err = os.Create(targetConfigFilePath)
	assert.NoError(t, err)
	f.Close()
	configFilePath, workingDir, err = findConfigFilePaths(baseDir, targetPath)
	assert.NoError(t, err)
	assert.Equal(t, targetConfigFilePath, configFilePath)
	assert.Equal(t, targetPath, workingDir)

	// we can use the config file itself as a target path
	configFilePath, workingDir, err = findConfigFilePaths(baseDir, targetConfigFilePath)
	assert.NoError(t, err)
	assert.Equal(t, targetConfigFilePath, configFilePath)
	assert.Equal(t, targetPath, workingDir)
}

func TestSplitConfigFilesAndRawManifestPaths(t *testing.T) {
	baseDir, clean := testfiles.TempDir(t)
	defer clean()

	targets := []string{
		filepath.Join(baseDir, "envs/staging"),
		filepath.Join(baseDir, "envs/production"),
		filepath.Join(baseDir, "commonresources"),
	}
	for _, target := range targets {
		err := os.MkdirAll(target, 0777)
		assert.NoError(t, err)
	}

	// create common config file for the environments
	configFile := `---
version: 1
commandUpdated:
  generators: 
    - command: echo g1
`
	err := ioutil.WriteFile(filepath.Join(baseDir, "envs", ConfigFilename), []byte(configFile), 0700)
	assert.NoError(t, err)

	configFiles, rawManifestFiles, err := splitConfigFilesAndRawManifestPaths(baseDir, targets)
	assert.NoError(t, err)

	assert.Len(t, rawManifestFiles, 1)
	assert.Equal(t, filepath.Join(baseDir, "commonresources"), rawManifestFiles[0])

	assert.Len(t, configFiles, 2)
	// We assume config files are processed in order to simplify the checks
	assert.Equal(t, filepath.Join(baseDir, "envs/staging"), configFiles[0].WorkingDir)
	assert.Equal(t, filepath.Join(baseDir, "envs/production"), configFiles[1].WorkingDir)
	assert.NotNil(t, configFiles[0].CommandUpdated)
	assert.Len(t, configFiles[0].CommandUpdated.Generators, 1)
	assert.Equal(t, "echo g1", configFiles[0].CommandUpdated.Generators[0].Command)
	assert.NotNil(t, configFiles[1].CommandUpdated)
	assert.Equal(t, configFiles[0].CommandUpdated.Generators, configFiles[0].CommandUpdated.Generators)
}

func setup(t *testing.T, configFileBody string) (*configAware, func()) {
	manifests := kubernetes.NewManifests(kubernetes.ConstNamespacer("default"), log.NewLogfmtLogger(os.Stdout))
	baseDir, cleanup := testfiles.TempDir(t)
	if len(configFileBody) > 0 {
		ioutil.WriteFile(filepath.Join(baseDir, ConfigFilename), []byte(configFileBody), 0600)
	}
	frs, err := NewConfigAware(baseDir, []string{baseDir}, manifests)
	assert.NoError(t, err)
	return frs, cleanup
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
      policy:
        command: echo ua $FLUX_WORKLOAD
`

func TestCommandUpdatedConfigFile(t *testing.T) {
	frs, cleanup := setup(t, commandUpdatedEchoConfigFile)
	defer cleanup()
	ctx := context.Background()
	resources, err := frs.GetAllResourcesByID(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(resources))
	deploymentID := resource.MustParseID("default:deployment/helloworld")
	assert.Contains(t, resources, deploymentID.String())
	ref, err := image.ParseRef("repo/image:tag")
	assert.NoError(t, err)
	err = frs.SetWorkloadContainerImage(ctx, deploymentID, "greeter", ref)
	assert.NoError(t, err)
	_, err = frs.UpdateWorkloadPolicies(ctx, deploymentID, resource.PolicyUpdate{
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

func TestPatchUpdatedConfigFile(t *testing.T) {
	frs, cleanup := setup(t, patchUpdatedEchoConfigFile)
	defer cleanup()
	ctx := context.Background()
	resources, err := frs.GetAllResourcesByID(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(resources))
	var deployment resource.Resource
	deploymentID := resource.MustParseID("default:deployment/helloworld")
	for id, res := range resources {
		if id == deploymentID.String() {
			deployment = res
		}
	}
	assert.NotNil(t, deployment)
	ref, err := image.ParseRef("repo/image:tag")
	assert.NoError(t, err)
	err = frs.SetWorkloadContainerImage(ctx, deploymentID, "greeter", ref)
	assert.NoError(t, err)
	_, err = frs.UpdateWorkloadPolicies(ctx, deploymentID, resource.PolicyUpdate{
		Add: policy.Set{policy.TagPrefix("greeter"): "glob:master-*"},
	})
	expectedPatch := `---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  annotations:
    fluxcd.io/tag.greeter: glob:master-*
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
	patchFilePath := filepath.Join(frs.baseDir, "patchfile.yaml")
	patch, err := ioutil.ReadFile(patchFilePath)
	assert.NoError(t, err)
	assert.Equal(t, expectedPatch, string(patch))
}
