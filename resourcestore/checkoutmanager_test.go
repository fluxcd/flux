package resourcestore

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/flux/cluster/kubernetes/testfiles"
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
