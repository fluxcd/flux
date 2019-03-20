package resourcestore

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
)

type manifestFileManager struct {
	checkoutDir  string
	manifestDirs []string
	manifests    cluster.Manifests
}

var _ updatableResourceStore = &manifestFileManager{}

func (mfm *manifestFileManager) GetAllResources() ([]updatableResource, error) {
	resources, err := mfm.manifests.LoadManifests(mfm.checkoutDir, mfm.manifestDirs)
	if err != nil {
		return nil, err
	}
	result := []updatableResource{}
	for _, r := range resources {
		mr := &manifestFileResource{
			Resource: r,
			manager:  mfm,
		}
		result = append(result, mr)
	}
	return result, nil
}

type manifestFileResource struct {
	resource.Resource
	manager *manifestFileManager
}

func (r *manifestFileResource) SetWorkloadContainerImage(container string, newImageID image.Ref) error {
	fullFilePath := filepath.Join(r.manager.checkoutDir, r.Source())
	def, err := ioutil.ReadFile(fullFilePath)
	if err != nil {
		return err
	}
	newDef, err := r.manager.manifests.SetWorkloadContainerImage(def, r.ResourceID(), container, newImageID)
	if err != nil {
		return err
	}
	fi, err := os.Stat(fullFilePath)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(fullFilePath, newDef, fi.Mode()); err != nil {
		return err
	}
	return nil
}

func (r *manifestFileResource) UpdateWorkloadPolicies(update policy.Update) (bool, error) {
	fullFilePath := filepath.Join(r.manager.checkoutDir, r.Source())
	def, err := ioutil.ReadFile(fullFilePath)
	if err != nil {
		return false, err
	}
	newDef, err := r.manager.manifests.UpdateWorkloadPolicies(def, r.ResourceID(), update)
	if err != nil {
		return false, err
	}
	fi, err := os.Stat(fullFilePath)
	if err != nil {
		return false, err
	}
	if err := ioutil.WriteFile(fullFilePath, newDef, fi.Mode()); err != nil {
		return false, err
	}
	return bytes.Compare(def, newDef) != 0, nil
}

func (r *manifestFileResource) GetResource() resource.Resource {
	return r.Resource
}
