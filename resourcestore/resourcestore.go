package resourcestore

import (
	"fmt"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
)

type ResourceStoreError struct {
	error
}

func ErrResourceNotFound(name string) error {
	return ResourceStoreError{fmt.Errorf("resource %s not found", name)}
}

// ResourceStore manages all the cluster resources defined in a checked out repository, explicitly declared
// in a file or not e.g., generated and updated by a .flux.yaml file, explicit Kubernetes .yaml manifests files ...
type ResourceStore interface {
	// Set the container image of a resource in the store
	SetWorkloadContainerImage(resourceID flux.ResourceID, container string, newImageID image.Ref) error
	// UpdateWorkloadPolicies modifies a resource in the store to apply the policy-update specified.
	// It returns whether a change in the resource was actually made as a result of the change
	UpdateWorkloadPolicies(flux.ResourceID, policy.Update) (bool, error)
	// Load all the resources in the store. The returned map is indexed by the resource IDs
	GetAllResourcesByID() (map[string]resource.Resource, error)
	// Load all the resources in the store. The returned map is indexed by the file which defines de resource
	GetAllResourcesBySource() (map[string]resource.Resource, error)
}

// updatableResource is a Resource whose container images and policies can be updated
type updatableResource interface {
	GetResource() resource.Resource
	SetWorkloadContainerImage(container string, newImageID image.Ref) error
	UpdateWorkloadPolicies(policy.Update) (bool, error)
}

// updatableResourceStore is a package-internal version of ResourceStore, abstracting out update operations
type updatableResourceStore interface {
	GetAllResources() ([]updatableResource, error)
}
