package resourcestore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
)

type resourceMaps struct {
	resourcesBySource map[string]updatableResource
	resourcesByID     map[string]updatableResource
}

type checkoutManager struct {
	ctx              context.Context
	checkout         *git.Checkout
	policyTranslator cluster.PolicyTranslator
	resourceStores   []updatableResourceStore
	cache            *resourceMaps
	sync.RWMutex
}

var _ ResourceStore = &checkoutManager{}

func NewCheckoutManager(ctx context.Context, enableManifestGeneration bool,
	manifests cluster.Manifests, policyTranslator cluster.PolicyTranslator, checkout *git.Checkout) (*checkoutManager, error) {
	var (
		err             error
		configFiles     []*ConfigFile
		rawManifestDirs []string
	)

	rawManifestDirs = checkout.ManifestDirs()
	if enableManifestGeneration {
		configFiles, rawManifestDirs, err = splitConfigFilesAndRawManifestPaths(checkout.Dir(), checkout.ManifestDirs())
		if err != nil {
			return nil, err
		}
	}

	result := &checkoutManager{
		checkout: checkout,
		ctx:      ctx,
	}
	if len(rawManifestDirs) > 0 {
		mfm := &manifestFileManager{
			checkoutDir:  checkout.Dir(),
			manifestDirs: rawManifestDirs,
			manifests:    manifests,
		}
		result.resourceStores = append(result.resourceStores, mfm)
	}
	for _, cf := range configFiles {
		relConfigFilePath, err := filepath.Rel(checkout.Dir(), cf.Path)
		if err != nil {
			return nil, err
		}
		cfm := &configFileManager{
			ctx:                  ctx,
			checkout:             checkout,
			configFile:           cf,
			publicConfigFilePath: relConfigFilePath,
			manifests:            manifests,
			policyTranslator:     policyTranslator,
		}
		result.resourceStores = append(result.resourceStores, cfm)
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
			if err != nil {
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
	_, cleanInitialPath, err := cleanAndEnsurePaternity(baseDir, initialPath)
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

func (cm *checkoutManager) SetWorkloadContainerImage(id flux.ResourceID, container string, newImageID image.Ref) error {
	maps, err := cm.getResourceMaps()
	if err != nil {
		return err
	}
	res, ok := maps.resourcesByID[id.String()]
	if !ok {
		return ErrResourceNotFound(id.String())
	}
	if err := res.SetWorkloadContainerImage(container, newImageID); err != nil {
		return err
	}
	// Reset cached resources, since we have modified one
	cm.resetCache()
	return nil
}

func (cm *checkoutManager) UpdateWorkloadPolicies(id flux.ResourceID, update policy.Update) (bool, error) {
	maps, err := cm.getResourceMaps()
	if err != nil {
		return false, err
	}
	res, ok := maps.resourcesByID[id.String()]
	if !ok {
		return false, ErrResourceNotFound(id.String())
	}
	changed, err := res.UpdateWorkloadPolicies(update)
	if err != nil {
		return false, err
	}
	// Reset cached resources, since we have modified one
	cm.resetCache()
	return changed, nil
}

func (cm *checkoutManager) GetAllResourcesByID() (map[string]resource.Resource, error) {
	maps, err := cm.getResourceMaps()
	if err != nil {
		return nil, err
	}
	return toResourceMap(maps.resourcesByID), nil
}

func (cm *checkoutManager) GetAllResourcesBySource() (map[string]resource.Resource, error) {
	maps, err := cm.getResourceMaps()
	if err != nil {
		return nil, err
	}
	return toResourceMap(maps.resourcesBySource), nil
}

func toResourceMap(resourceMap map[string]updatableResource) map[string]resource.Resource {
	result := map[string]resource.Resource{}
	for k, r := range resourceMap {
		result[k] = r.GetResource()
	}
	return result
}

func (cm *checkoutManager) getResourceMaps() (*resourceMaps, error) {
	cm.RLock()
	if cm.cache != nil {
		cm.RUnlock()
		return cm.cache, nil
	}
	cm.RUnlock()
	resources := []updatableResource{}
	for _, rs := range cm.resourceStores {
		storeResources, err := rs.GetAllResources()
		if err != nil {
			return nil, err
		}
		resources = append(resources, storeResources...)
	}
	resourcesByID := map[string]updatableResource{}
	resourcesBySource := map[string]updatableResource{}
	for _, ur := range resources {
		r := ur.GetResource()
		resourcesByID[r.ResourceID().String()] = ur
		resourcesBySource[r.Source()] = ur
	}
	maps := &resourceMaps{
		resourcesByID:     resourcesByID,
		resourcesBySource: resourcesBySource,
	}
	cm.Lock()
	cm.cache = maps
	cm.Unlock()
	return maps, nil
}

func (cm *checkoutManager) resetCache() {
	cm.Lock()
	cm.cache = nil
	cm.Unlock()
}

func cleanAndEnsurePaternity(basePath string, childPath string) (string, string, error) {
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
