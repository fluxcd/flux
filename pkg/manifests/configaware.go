package manifests

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
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
	} else if err := ca.setConfigFileWorkloadContainerImage(ctx, resWithOrigin.configFile, resWithOrigin.resource, container, newImageID); err != nil {
		return err
	}
	// Reset resources, since we have modified one
	ca.resetResources()
	return nil
}

func (ca *configAware) setConfigFileWorkloadContainerImage(ctx context.Context, cf *ConfigFile, r resource.Resource,
	container string, newImageID image.Ref) error {
	if cf.PatchUpdated != nil {
		return ca.updatePatchFile(ctx, cf, func(previousManifests []byte) ([]byte, error) {
			return ca.manifests.SetWorkloadContainerImage(previousManifests, r.ResourceID(), container, newImageID)
		})
	}

	// Command-updated
	result := cf.ExecContainerImageUpdaters(ctx,
		r.ResourceID(),
		container,
		newImageID.Name.String(), newImageID.Tag,
	)
	if len(result) > 0 && result[len(result)-1].Error != nil {
		updaters := cf.CommandUpdated.Updaters
		return fmt.Errorf("error executing image updater command %q from file %q: %s\noutput:\n%s",
			updaters[len(result)-1].ContainerImage.Command,
			result[len(result)-1].Error,
			r.Source(),
			result[len(result)-1].Output,
		)
	}
	return nil
}

func (ca *configAware) updatePatchFile(ctx context.Context, cf *ConfigFile,
	updateF func(previousManifests []byte) ([]byte, error)) error {

	patchUpdated := *cf.PatchUpdated
	generatedManifests, patchedManifests, patchFilePath, err := ca.getGeneratedAndPatchedManifests(ctx, cf, patchUpdated)
	if err != nil {
		relConfigFilePath, err := filepath.Rel(ca.baseDir, cf.Path)
		if err != nil {
			return err
		}
		return fmt.Errorf("error parsing generated, patched output from file %s: %s", relConfigFilePath, err)
	}
	finalManifests, err := updateF(patchedManifests)
	if err != nil {
		return err
	}
	newPatch, err := ca.manifests.CreateManifestPatch(generatedManifests, finalManifests,
		"generated manifests", "patched and updated manifests")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(patchFilePath, newPatch, 0600)
}

func (ca *configAware) getGeneratedAndPatchedManifests(ctx context.Context, cf *ConfigFile, patchUpdated PatchUpdated) ([]byte, []byte, string, error) {
	generatedManifests, err := ca.getGeneratedManifests(ctx, cf, patchUpdated.Generators)
	if err != nil {
		return nil, nil, "", err
	}

	// The patch file is expressed relatively to the configuration file's working directory
	explicitPatchFilePath := patchUpdated.PatchFile
	patchFilePath := filepath.Join(cf.WorkingDir, explicitPatchFilePath)

	// Make sure that the patch file doesn't fall out of the Git repository checkout
	_, _, err = cleanAndEnsureParentPath(ca.baseDir, patchFilePath)
	if err != nil {
		return nil, nil, "", err
	}
	patch, err := ioutil.ReadFile(patchFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, nil, "", err
		}
		// Tolerate a missing patch file, since it may not have been created yet.
		// However, its base path must exist.
		patchBaseDir := filepath.Dir(patchFilePath)
		if stat, err := os.Stat(patchBaseDir); err != nil || !stat.IsDir() {
			err := fmt.Errorf("base directory (%q) of patchFile (%q) does not exist",
				filepath.Dir(explicitPatchFilePath), explicitPatchFilePath)
			return nil, nil, "", err
		}
		patch = nil
	}
	relConfigFilePath, err := filepath.Rel(ca.baseDir, cf.Path)
	if err != nil {
		return nil, nil, "", err
	}
	patchedManifests, err := ca.manifests.ApplyManifestPatch(generatedManifests, patch, relConfigFilePath, explicitPatchFilePath)
	if err != nil {
		return nil, nil, "", fmt.Errorf("cannot patch generated resources: %s", err)
	}
	return generatedManifests, patchedManifests, patchFilePath, nil
}

func (ca *configAware) getGeneratedManifests(ctx context.Context, cf *ConfigFile, generators []Generator) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	for i, cmdResult := range cf.ExecGenerators(ctx, generators) {
		relConfigFilePath, err := filepath.Rel(ca.baseDir, cf.Path)
		if err != nil {
			return nil, err
		}
		if cmdResult.Error != nil {
			err := fmt.Errorf("error executing generator command %q from file %q: %s\nerror output:\n%s\ngenerated output:\n%s",
				generators[i].Command,
				relConfigFilePath,
				cmdResult.Error,
				string(cmdResult.Stderr),
				string(cmdResult.Stderr),
			)
			return nil, err
		}
		if err := ca.manifests.AppendManifestToBuffer(cmdResult.Stdout, buf); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func (ca *configAware) UpdateWorkloadPolicies(ctx context.Context, resourceID resource.ID,
	update resource.PolicyUpdate) (bool, error) {
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
		changed, err = ca.updateConfigFileWorkloadPolicies(ctx, resWithOrigin.configFile, resWithOrigin.resource, update)
	}
	if err != nil {
		return false, err
	}
	// Reset resources, since we have modified one
	ca.resetResources()
	return changed, nil
}

func (ca *configAware) updateConfigFileWorkloadPolicies(ctx context.Context, cf *ConfigFile, r resource.Resource,
	update resource.PolicyUpdate) (bool, error) {
	if cf.PatchUpdated != nil {
		var changed bool
		err := ca.updatePatchFile(ctx, cf, func(previousManifests []byte) ([]byte, error) {
			updatedManifests, err := ca.manifests.UpdateWorkloadPolicies(previousManifests, r.ResourceID(), update)
			if err == nil {
				changed = bytes.Compare(previousManifests, updatedManifests) != 0
			}
			return updatedManifests, err
		})
		return changed, err
	}

	// Command-updated
	workload, ok := r.(resource.Workload)
	if !ok {
		return false, errors.New("resource " + r.ResourceID().String() + " does not have containers")
	}
	changes, err := resource.ChangesForPolicyUpdate(workload, update)
	if err != nil {
		return false, err
	}

	for key, value := range changes {
		result := cf.ExecPolicyUpdaters(ctx, r.ResourceID(), key, value)
		if len(result) > 0 && result[len(result)-1].Error != nil {
			updaters := cf.CommandUpdated.Updaters
			err := fmt.Errorf("error executing annotation updater command %q from file %q: %s\noutput:\n%s",
				updaters[len(result)-1].Policy.Command,
				result[len(result)-1].Error,
				r.Source(),
				result[len(result)-1].Output,
			)
			return false, err
		}
	}
	// We assume that the update changed the resource. Alternatively, we could generate the resources
	// again and compare the output, but that's expensive.
	return true, nil
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
		var (
			resourceManifests []byte
			err               error
		)
		if cf.CommandUpdated != nil {
			var err error
			resourceManifests, err = ca.getGeneratedManifests(ctx, cf, cf.CommandUpdated.Generators)
			if err != nil {
				return nil, err
			}
		} else {
			_, resourceManifests, _, err = ca.getGeneratedAndPatchedManifests(ctx, cf, *cf.PatchUpdated)
		}
		if err != nil {
			return nil, err
		}
		relConfigFilePath, err := filepath.Rel(ca.baseDir, cf.Path)
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
