package manifests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/resource"
)

type resourceWithOrigin struct {
	resource   resource.Resource
	configFile *ConfigFile // only set if the resource came from a configuration file
}

type configAware struct {
	// rawFiles will do everything for the paths that have no config file
	rawFiles *rawFiles

	// to maintain encapsulation, we don't rely on the rawFiles values
	baseDir     string
	manifests   Manifests
	configFiles []*ConfigFile

	// a cache of the loaded resources, since the pattern is to update
	// a few things at a time, and the update operations all need to
	// have a set of resources
	mu            sync.RWMutex
	resourcesByID map[string]resourceWithOrigin
}

func NewConfigAware(baseDir string, targetPaths []string, manifests Manifests) (*configAware, error) {
	configFiles, rawManifestDirs, err := splitConfigFilesAndRawManifestPaths(baseDir, targetPaths)
	if err != nil {
		return nil, err
	}

	result := &configAware{
		rawFiles: &rawFiles{
			manifests: manifests,
			baseDir:   baseDir,
			paths:     rawManifestDirs,
		},
		manifests:   manifests,
		baseDir:     baseDir,
		configFiles: configFiles,
	}
	return result, nil
}

func splitConfigFilesAndRawManifestPaths(baseDir string, paths []string) ([]*ConfigFile, []string, error) {
	var (
		configFiles      []*ConfigFile
		rawManifestPaths []string
	)

	for _, path := range paths {
		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return nil, nil, err
		}
		configFilePath, workingDirPath, err := findConfigFilePaths(baseDir, path)
		if err != nil {
			if err == configFileNotFoundErr {
				rawManifestPaths = append(rawManifestPaths, path)
				continue
			}
			return nil, nil, fmt.Errorf("error when searching config files for path %q: %s", relPath, err)
		}
		cf, err := NewConfigFile(configFilePath, workingDirPath)
		if err != nil {
			relConfigFilePath, relErr := filepath.Rel(baseDir, configFilePath)
			if relErr != nil {
				return nil, nil, relErr
			}
			return nil, nil, fmt.Errorf("cannot parse config file %q: %s", relConfigFilePath, err)
		}
		configFiles = append(configFiles, cf)
	}

	return configFiles, rawManifestPaths, nil
}

var configFileNotFoundErr = fmt.Errorf("config file not found")

func findConfigFilePaths(baseDir string, initialPath string) (string, string, error) {
	// The path can directly be a .flux.yaml config file
	fileStat, err := os.Stat(initialPath)
	if err != nil {
		return "", "", err
	}
	if !fileStat.IsDir() {
		workingDir, filename := filepath.Split(initialPath)
		if filename == ConfigFilename {
			return initialPath, filepath.Clean(workingDir), nil
		}
		return "", "", configFileNotFoundErr
	}

	// Make paths canonical and remove potential ending slash,
	// for filepath.Dir() to work as we expect.
	// Also, the initial path must be contained in baseDir
	// (to make sure we don't escape the git checkout when
	// moving upwards in the directory hierarchy)
	_, cleanInitialPath, err := cleanAndEnsureParentPath(baseDir, initialPath)
	if err != nil {
		return "", "", err
	}

	for path := cleanInitialPath; ; {
		potentialConfigFilePath := filepath.Join(path, ConfigFilename)
		if _, err := os.Stat(potentialConfigFilePath); err == nil {
			return potentialConfigFilePath, initialPath, nil
		}
		if path == baseDir {
			break
		}
		// check the parent directory
		path = filepath.Dir(path)
	}

	return "", "", configFileNotFoundErr
}

func (ca *configAware) SetWorkloadContainerImage(ctx context.Context, resourceID resource.ID, container string,
	newImageID image.Ref) error {
	resourcesByID, err := ca.getResourcesByID(ctx)
	if err != nil {
		return err
	}
	resWithOrigin, ok := resourcesByID[resourceID.String()]
	if !ok {
		return ErrResourceNotFound(resourceID.String())
	}
	if resWithOrigin.configFile == nil {
		if err := ca.rawFiles.setManifestWorkloadContainerImage(resWithOrigin.resource, container, newImageID); err != nil {
			return err
		}
	} else if err := resWithOrigin.configFile.SetWorkloadContainerImage(ctx, ca.manifests, resWithOrigin.resource, container, newImageID); err != nil {
		return err
	}
	// Reset resources, since we have modified one
	ca.resetResources()
	return nil
}

func (ca *configAware) UpdateWorkloadPolicies(ctx context.Context, resourceID resource.ID, update resource.PolicyUpdate) (bool, error) {
	resourcesByID, err := ca.getResourcesByID(ctx)
	if err != nil {
		return false, err
	}
	resWithOrigin, ok := resourcesByID[resourceID.String()]
	if !ok {
		return false, ErrResourceNotFound(resourceID.String())
	}
	var changed bool
	if resWithOrigin.configFile == nil {
		changed, err = ca.rawFiles.updateManifestWorkloadPolicies(resWithOrigin.resource, update)
	} else {
		cf := resWithOrigin.configFile
		changed, err = cf.UpdateWorkloadPolicies(ctx, ca.manifests, resWithOrigin.resource, update)
	}
	if err != nil {
		return false, err
	}
	// Reset resources, since we have modified one
	ca.resetResources()
	return changed, nil
}

func (ca *configAware) GetAllResourcesByID(ctx context.Context) (map[string]resource.Resource, error) {
	resourcesByID, err := ca.getResourcesByID(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[string]resource.Resource, len(resourcesByID))
	for id, resourceWithOrigin := range resourcesByID {
		result[id] = resourceWithOrigin.resource
	}
	return result, nil
}

func (ca *configAware) getResourcesByID(ctx context.Context) (map[string]resourceWithOrigin, error) {
	ca.mu.RLock()
	if ca.resourcesByID != nil {
		toReturn := ca.resourcesByID
		ca.mu.RUnlock()
		return toReturn, nil
	}
	ca.mu.RUnlock()

	resourcesByID := map[string]resourceWithOrigin{}

	rawResourcesByID, err := ca.rawFiles.GetAllResourcesByID(ctx)
	if err != nil {
		return nil, err
	}
	for id, res := range rawResourcesByID {
		resourcesByID[id] = resourceWithOrigin{resource: res}
	}

	for _, cf := range ca.configFiles {
		resourceManifests, err := cf.GenerateManifests(ctx, ca.manifests)
		if err != nil {
			return nil, err
		}
		relConfigFilePath, err := cf.RelativeConfigPath()
		if err != nil {
			return nil, err
		}
		resources, err := ca.manifests.ParseManifest(resourceManifests, relConfigFilePath)
		if err != nil {
			return nil, err
		}
		for id, r := range resources {
			if _, ok := resourcesByID[id]; ok {
				return nil, fmt.Errorf("duplicate resource from %s and %s",
					r.Source(), resourcesByID[id].resource.Source())
			}
			resourcesByID[id] = resourceWithOrigin{resource: r, configFile: cf}
		}
	}
	ca.mu.Lock()
	ca.resourcesByID = resourcesByID
	ca.mu.Unlock()
	return resourcesByID, nil
}

func (ca *configAware) resetResources() {
	ca.mu.Lock()
	ca.resourcesByID = nil
	ca.mu.Unlock()
}

func cleanAndEnsureParentPath(basePath string, childPath string) (string, string, error) {
	// Make paths canonical and remove potential ending slash,
	// for filepath.Dir() to work as we expect
	cleanBasePath, err := filepath.Abs(basePath)
	if err != nil {
		return "", "", err
	}
	cleanChildPath, err := filepath.Abs(childPath)
	if err != nil {
		return "", "", err
	}
	cleanBasePath = filepath.Clean(cleanBasePath)
	cleanChildPath = filepath.Clean(cleanChildPath)

	// The initial path must be relative to baseDir
	// (to make sure we don't escape the git checkout when
	// moving upwards in the directory hierarchy)
	if !strings.HasPrefix(cleanChildPath, cleanBasePath) {
		return "", "", fmt.Errorf("path %q is outside of base directory %s", childPath, basePath)
	}
	return cleanBasePath, cleanChildPath, nil
}
