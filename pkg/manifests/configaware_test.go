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

// represents, for the purpose of setting a test up, either a
// - path to a .flux.yaml: `config{path: ..., fluxyaml: ...}`
// - a .flux.yaml for the whole "repo": `config{fluxyaml: ...}`
type config struct {
	path     string
	fluxyaml string
}

// set a directory up with the given `--git-path` arguments
// (subpaths), and locations in which to put config files. The paths
// and the config locations don't necessarily have to line up. You can
// pass `nil` for the paths, to indicate "just use the base path".
func setup(t *testing.T, paths []string, configs ...config) (*configAware, string, func()) {
	manifests := kubernetes.NewManifests(kubernetes.ConstNamespacer("default"), log.NewLogfmtLogger(os.Stdout))
	baseDir, cleanup := testfiles.TempDir(t)

	// te constructor NewConfigAware expects at least one absolute path.
	var searchPaths []string
	for _, p := range paths {
		searchPaths = append(searchPaths, filepath.Join(baseDir, p))
	}
	if len(paths) == 0 {
		searchPaths = []string{baseDir}
	}

	for _, c := range configs {
		p := c.path
		if p == "" {
			p = "."
		}
		if len(c.fluxyaml) > 0 {
			err := os.MkdirAll(filepath.Join(baseDir, p), 0777)
			assert.NoError(t, err)
			ioutil.WriteFile(filepath.Join(baseDir, p, ConfigFilename), []byte(c.fluxyaml), 0600)
		}
	}
	frs, err := NewConfigAware(baseDir, searchPaths, manifests)
	assert.NoError(t, err)
	return frs, baseDir, cleanup
}

// ---

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
	assert.Equal(t, filepath.Join(baseDir, "envs/staging"), configFiles[0].workingDir)
	assert.Equal(t, filepath.Join(baseDir, "envs/production"), configFiles[1].workingDir)

	assert.Equal(t, "envs/staging/../.flux.yaml", configFiles[0].ConfigRelativeToWorkingDir())
	assert.Equal(t, "envs/production/../.flux.yaml", configFiles[1].ConfigRelativeToWorkingDir())

	assert.NotNil(t, configFiles[0].CommandUpdated)
	assert.Len(t, configFiles[0].CommandUpdated.Generators, 1)
	assert.Equal(t, "echo g1", configFiles[0].CommandUpdated.Generators[0].Command)
	assert.NotNil(t, configFiles[1].CommandUpdated)
	assert.Equal(t, configFiles[0].CommandUpdated.Generators, configFiles[0].CommandUpdated.Generators)
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
	frs, _, cleanup := setup(t, nil, config{fluxyaml: commandUpdatedEchoConfigFile})
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
	frs, _, cleanup := setup(t, nil, config{fluxyaml: patchUpdatedEchoConfigFile})
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

const mistakenConf = `
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
               image: quay.io/weaveworks/helloworld:master-a000001
       ---
       apiVersion: v1
       kind: Namespace
       metadata:
         name: demo"
`

// This tests that when using a config with no update commands, and
// update operation results in an error, rather than a silent failure
// to make any changes.
func TestMistakenConfigFile(t *testing.T) {
	frs, _, cleanup := setup(t, nil, config{fluxyaml: mistakenConf})
	defer cleanup()

	deploymentID := resource.MustParseID("default:deployment/helloworld")
	ref, _ := image.ParseRef("repo/image:tag")

	ctx := context.Background()
	err := frs.SetWorkloadContainerImage(ctx, deploymentID, "greeter", ref)
	assert.Error(t, err)
}

const duplicateGeneration = `
version: 1
commandUpdated:
  generators:
  - command: |
     echo "apiVersion: v1
     kind: Namespace
     metadata:
       name: demo
     ---
     apiVersion: extensions/v1beta1
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
       name: demo
     "
`

func TestDuplicateDetection(t *testing.T) {
	// this one has the same resource twice in the generated manifests
	conf, _, cleanup := setup(t, nil, config{fluxyaml: duplicateGeneration})
	defer cleanup()

	res, err := conf.GetAllResourcesByID(context.Background())
	assert.Nil(t, res)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

// A de-indented version of the deployment manifest echoed in the
// patchUpdatedEchoConfigFile above, to be written to a file.
const helloManifest = `
apiVersion: extensions/v1beta1
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
`

func TestDuplicateInFiles(t *testing.T) {
	// this one tests that a manifest that's in a file as well as generated is detected as a duplicate
	frs, baseDir, cleanup := setup(t, []string{".", "echo"}, config{path: "echo", fluxyaml: patchUpdatedEchoConfigFile})
	defer cleanup()
	ioutil.WriteFile(filepath.Join(baseDir, "manifest.yaml"), []byte(helloManifest), 0666)

	res, err := frs.GetAllResourcesByID(context.Background())
	assert.Nil(t, res)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestDuplicateInGenerators(t *testing.T) {
	// this one tests that a manifest that's generated by two different generator configs
	frs, _, cleanup := setup(t, []string{"echo1", "echo2"},
		config{path: "echo1", fluxyaml: patchUpdatedEchoConfigFile},
		config{path: "echo2", fluxyaml: patchUpdatedEchoConfigFile})
	defer cleanup()

	res, err := frs.GetAllResourcesByID(context.Background())
	assert.Nil(t, res)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestSccanForFiles(t *testing.T) {
	// +-- config
	//   +-- .flux.yaml (patchUpdated)
	//   +-- rawfiles
	//     +-- .flux.yaml (scanForFiles)
	//     +-- manifest.yaml

	manifestyaml := `
apiVersion: v1
kind: Namespace
metadata:
  name: foo-ns
`

	config, baseDir, cleanup := setup(t, []string{filepath.Join("config", "rawfiles")},
		config{path: "config", fluxyaml: patchUpdatedEchoConfigFile},
		config{path: filepath.Join("config", "rawfiles"), fluxyaml: "version: 1\nscanForFiles: {}\n"},
	)
	defer cleanup()

	assert.NoError(t, ioutil.WriteFile(filepath.Join(baseDir, "config", "rawfiles", "manifest.yaml"), []byte(manifestyaml), 0600))

	res, err := config.GetAllResourcesByID(context.Background())
	assert.NoError(t, err)
	assert.Contains(t, res, "default:namespace/foo-ns")
}
