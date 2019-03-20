package resourcestore

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
)

type configFileManager struct {
	ctx              context.Context
	checkoutDir      string
	configFile       *ConfigFile
	manifests        cluster.Manifests
	policyTranslator cluster.PolicyTranslator
}

var _ updatableResourceStore = &configFileManager{}

func (cfm *configFileManager) GetAllResources() ([]updatableResource, error) {
	var result []updatableResource
	for i, cmdResult := range cfm.configFile.ExecGenerators(cfm.ctx) {
		relConfigFilePath, err := filepath.Rel(cfm.checkoutDir, filepath.Join(cfm.configFile.WorkingDir, ConfigFilename))
		if err != nil {
			return nil, err
		}
		errorf := func(err error) error {
			return fmt.Errorf("error executing generator command %q from file %q: %s\nerror output:\n%s\ngenerated output:\n%s",
				cfm.configFile.Generators[i].Command,
				relConfigFilePath,
				err,
				string(cmdResult.Stderr),
				string(cmdResult.Stderr),
			)
		}
		if cmdResult.Error != nil {
			return nil, errorf(cmdResult.Error)
		}
		resources, err := cfm.manifests.ParseManifest(cmdResult.Stdout, relConfigFilePath)
		if err != nil {
			return nil, errorf(err)
		}
		for _, r := range resources {
			g := &generatedResource{
				Resource: r,
				manager:  cfm,
			}
			result = append(result, g)
		}
	}
	return result, nil
}

type generatedResource struct {
	resource.Resource
	manager *configFileManager
}

var _ updatableResource = &generatedResource{}

func (gr *generatedResource) SetWorkloadContainerImage(container string, newImageID image.Ref) error {
	result := gr.manager.configFile.ExecContainerImageUpdaters(gr.manager.ctx,
		gr.ResourceID(),
		container,
		newImageID.Name.String(), newImageID.Tag,
	)
	if len(result) > 0 && result[len(result)-1].Error != nil {
		return fmt.Errorf("error executing image updater command %q from file %q: %s\noutput:\n%s",
			gr.manager.configFile.Updaters[len(result)-1].ContainerImage.Command,
			result[len(result)-1].Error,
			gr.Source(),
			result[len(result)-1].Output,
		)
	}
	return nil
}

func (gr *generatedResource) UpdateWorkloadPolicies(update policy.Update) (bool, error) {
	workload, ok := gr.Resource.(resource.Workload)
	if !ok {
		return false, errors.New("resource " + gr.ResourceID().String() + " does not have containers")
	}
	changes, err := gr.manager.policyTranslator.GetAnnotationChangesForPolicyUpdate(workload, update)
	if err != nil {
		return false, err
	}
	for _, change := range changes {
		result := gr.manager.configFile.ExecAnnotationUpdaters(gr.manager.ctx,
			gr.ResourceID(),
			change.AnnotationKey,
			change.AnnotationValue,
		)
		if len(result) > 0 && result[len(result)-1].Error != nil {
			err := fmt.Errorf("error executing annotation updater command %q from file %q: %s\noutput:\n%s",
				gr.manager.configFile.Updaters[len(result)-1].Annotation.Command,
				result[len(result)-1].Error,
				gr.Source(),
				result[len(result)-1].Output,
			)
			return false, err
		}
	}
	// We assume that the update changed the resource. Alternatively, we could generate the resources
	// again and compare the output, but that's expensive.
	return true, nil
}

func (gr *generatedResource) GetResource() resource.Resource {
	return gr.Resource
}
