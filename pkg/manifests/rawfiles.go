package manifests

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/resource"
)

type rawFiles struct {
	baseDir   string
	paths     []string
	manifests Manifests
}

// NewRawFiles constructs a `Store` that assumes the provided
// directories contain plain YAML files
func NewRawFiles(baseDir string, paths []string, manifests Manifests) *rawFiles {
	return &rawFiles{
		baseDir:   baseDir,
		paths:     paths,
		manifests: manifests,
	}
}

// Set the container image of a resource in the store
func (f *rawFiles) SetWorkloadContainerImage(ctx context.Context, id resource.ID, container string, newImageID image.Ref) error {
	resourcesByID, err := f.GetAllResourcesByID(ctx)
	if err != nil {
		return err
	}
	r, ok := resourcesByID[id.String()]
	if !ok {
		return ErrResourceNotFound(id.String())
	}
	return f.setManifestWorkloadContainerImage(r, container, newImageID)
}

func (f *rawFiles) setManifestWorkloadContainerImage(r resource.Resource, container string, newImageID image.Ref) error {
	fullFilePath := filepath.Join(f.baseDir, r.Source())
	def, err := ioutil.ReadFile(fullFilePath)
	if err != nil {
		return err
	}
	newDef, err := f.manifests.SetWorkloadContainerImage(def, r.ResourceID(), container, newImageID)
	if err != nil {
		return err
	}
	fi, err := os.Stat(fullFilePath)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(fullFilePath, newDef, fi.Mode())
}

// UpdateWorkloadPolicies modifies a resource in the store to apply the policy-update specified.
// It returns whether a change in the resource was actually made as a result of the change
func (f *rawFiles) UpdateWorkloadPolicies(ctx context.Context, id resource.ID, update resource.PolicyUpdate) (bool, error) {
	resourcesByID, err := f.GetAllResourcesByID(ctx)
	if err != nil {
		return false, err
	}
	r, ok := resourcesByID[id.String()]
	if !ok {
		return false, ErrResourceNotFound(id.String())
	}
	return f.updateManifestWorkloadPolicies(r, update)
}

func (f *rawFiles) updateManifestWorkloadPolicies(r resource.Resource, update resource.PolicyUpdate) (bool, error) {
	fullFilePath := filepath.Join(f.baseDir, r.Source())
	def, err := ioutil.ReadFile(fullFilePath)
	if err != nil {
		return false, err
	}
	newDef, err := f.manifests.UpdateWorkloadPolicies(def, r.ResourceID(), update)
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

// Load all the resources in the store. The returned map is indexed by the resource IDs
func (f *rawFiles) GetAllResourcesByID(_ context.Context) (map[string]resource.Resource, error) {
	return f.manifests.LoadManifests(f.baseDir, f.paths)
}
