package resourcestore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
)

type configFileManager struct {
	ctx                  context.Context
	checkout             *git.Checkout
	configFile           *ConfigFile
	publicConfigFilePath string // for error reporting
	manifests            cluster.Manifests
	policyTranslator     cluster.PolicyTranslator
}

var _ updatableResourceStore = &configFileManager{}

func (cfm *configFileManager) GetAllResources() ([]updatableResource, error) {
	if cfm.configFile.CommandUpdated != nil {
		return cfm.getAllCommandUpdatedResources(*cfm.configFile.CommandUpdated)
	}
	return cfm.getAllPatchUpdatedResources(*cfm.configFile.PatchUpdated)
}

func (cfm *configFileManager) getAllCommandUpdatedResources(commandUpdated CommandUpdated) ([]updatableResource, error) {
	generatedManifests, err := cfm.getGeneratedManifests(commandUpdated.Generators)
	if err != nil {
		return nil, err
	}
	var result []updatableResource
	resources, err := cfm.manifests.ParseManifest(generatedManifests, cfm.publicConfigFilePath)
	if err != nil {
		return nil, fmt.Errorf("error parsing generated output from file %s: %s", cfm.publicConfigFilePath, err)
	}
	for _, r := range resources {
		cur := &commandUpdatedResource{
			Resource: r,
			cfm:      cfm,
		}
		result = append(result, cur)
	}
	return result, nil
}

func (cfm *configFileManager) getGeneratedManifests(generators []Generator) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	for i, cmdResult := range cfm.configFile.ExecGenerators(cfm.ctx, generators) {
		if cmdResult.Error != nil {
			err := fmt.Errorf("error executing generator command %q from file %q: %s\nerror output:\n%s\ngenerated output:\n%s",
				generators[i].Command,
				cfm.publicConfigFilePath,
				cmdResult.Error,
				string(cmdResult.Stderr),
				string(cmdResult.Stderr),
			)
			return nil, err
		}
		if err := cluster.AppendManifestToBuffer(cmdResult.Stdout, buf); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

type commandUpdatedResource struct {
	resource.Resource
	cfm *configFileManager
}

var _ updatableResource = &commandUpdatedResource{}

func (cur *commandUpdatedResource) GetResource() resource.Resource {
	return cur.Resource
}

func (cur *commandUpdatedResource) SetWorkloadContainerImage(container string, newImageID image.Ref) error {
	result := cur.cfm.configFile.ExecContainerImageUpdaters(cur.cfm.ctx,
		cur.ResourceID(),
		container,
		newImageID.Name.String(), newImageID.Tag,
	)
	if len(result) > 0 && result[len(result)-1].Error != nil {
		updaters := cur.cfm.configFile.CommandUpdated.Updaters
		return fmt.Errorf("error executing image updater command %q from file %q: %s\noutput:\n%s",
			updaters[len(result)-1].ContainerImage.Command,
			result[len(result)-1].Error,
			cur.Source(),
			result[len(result)-1].Output,
		)
	}
	return nil
}

func (cur *commandUpdatedResource) UpdateWorkloadPolicies(update policy.Update) (bool, error) {
	workload, ok := cur.Resource.(resource.Workload)
	if !ok {
		return false, errors.New("resource " + cur.ResourceID().String() + " does not have containers")
	}
	changes, err := cur.cfm.policyTranslator.GetAnnotationChangesForPolicyUpdate(workload, update)
	if err != nil {
		return false, err
	}
	for _, change := range changes {
		result := cur.cfm.configFile.ExecAnnotationUpdaters(cur.cfm.ctx,
			cur.ResourceID(),
			change.AnnotationKey,
			change.AnnotationValue,
		)
		if len(result) > 0 && result[len(result)-1].Error != nil {
			updaters := cur.cfm.configFile.CommandUpdated.Updaters
			err := fmt.Errorf("error executing annotation updater command %q from file %q: %s\noutput:\n%s",
				updaters[len(result)-1].Annotation.Command,
				result[len(result)-1].Error,
				cur.Source(),
				result[len(result)-1].Output,
			)
			return false, err
		}
	}
	// We assume that the update changed the resource. Alternatively, we could generate the resources
	// again and compare the output, but that's expensive.
	return true, nil
}

func (cfm *configFileManager) getAllPatchUpdatedResources(patchUpdated PatchUpdated) ([]updatableResource, error) {
	_, patchedManifests, _, err := cfm.getGeneratedAndPatchedManifests(patchUpdated)
	if err != nil {
		return nil, fmt.Errorf("error parsing generated, patched output from file %s: %s",
			cfm.publicConfigFilePath, err)
	}
	resources, err := cfm.manifests.ParseManifest(patchedManifests, cfm.publicConfigFilePath)
	if err != nil {
		return nil, err
	}
	var result []updatableResource
	for _, r := range resources {
		pur := &patchUpdatedResource{
			Resource: r,
			cfm:      cfm,
		}
		result = append(result, pur)
	}

	return result, nil
}

func (cfm *configFileManager) getGeneratedAndPatchedManifests(patchUpdated PatchUpdated) ([]byte, []byte, string, error) {
	generatedManifests, err := cfm.getGeneratedManifests(patchUpdated.Generators)
	if err != nil {
		return nil, nil, "", err
	}

	// The patch file is expressed relatively to the configuration file's working directory
	explicitPatchFilePath := patchUpdated.PatchFile
	patchFilePath := filepath.Join(cfm.configFile.WorkingDir, explicitPatchFilePath)

	// Make sure that the patch file doesn't fall out of the Git repository checkout
	_, _, err = cleanAndEnsurePaternity(cfm.checkout.Dir(), patchFilePath)
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
	patchedManifests, err := cfm.manifests.ApplyManifestPatch(generatedManifests, patch, cfm.publicConfigFilePath, explicitPatchFilePath)
	if err != nil {
		return nil, nil, "", fmt.Errorf("cannot patch generated resources: %s", err)
	}
	return generatedManifests, patchedManifests, patchFilePath, nil
}

type patchUpdatedResource struct {
	resource.Resource
	cfm *configFileManager
}

func (pur *patchUpdatedResource) GetResource() resource.Resource {
	return pur.Resource
}

func (pur *patchUpdatedResource) SetWorkloadContainerImage(container string, newImageID image.Ref) error {
	return pur.updatePatchFile(func(previousManifests []byte) ([]byte, error) {
		return pur.cfm.manifests.SetWorkloadContainerImage(previousManifests, pur.Resource.ResourceID(), container, newImageID)
	})
}

func (pur *patchUpdatedResource) UpdateWorkloadPolicies(update policy.Update) (bool, error) {
	err := pur.updatePatchFile(func(previousManifests []byte) ([]byte, error) {
		return pur.cfm.manifests.UpdateWorkloadPolicies(previousManifests, pur.Resource.ResourceID(), update)
	})
	// We assume that the update changed the patch file. Alternatively, we could compare the patch files.
	return true, err
}

func (pur *patchUpdatedResource) updatePatchFile(updateF func(previousManifests []byte) ([]byte, error)) error {
	patchUpdated := *pur.cfm.configFile.PatchUpdated
	generatedManifests, patchedManifests, patchFilePath, err := pur.cfm.getGeneratedAndPatchedManifests(patchUpdated)
	if err != nil {
		return fmt.Errorf("error parsing generated, patched output from file %s: %s", pur.cfm.publicConfigFilePath, err)
	}
	finalManifests, err := updateF(patchedManifests)
	newPatch, err := pur.cfm.manifests.CreateManifestPatch(generatedManifests, finalManifests,
		"generated manifests", "patched and updated manifests")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(patchFilePath, newPatch, 0600)
	if err != nil {
		return err
	}
	// We need to add the file to Git in case it's the first time there is a modification
	return pur.cfm.checkout.Add(pur.cfm.ctx, patchFilePath)
}

var _ updatableResource = &patchUpdatedResource{}
