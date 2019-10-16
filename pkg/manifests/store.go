package manifests

import (
	"context"
	"fmt"

	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/resource"
)

type StoreError struct {
	error
}

func ErrResourceNotFound(name string) error {
	return StoreError{fmt.Errorf("resource %s not found", name)}
}

// Store manages all the cluster resources defined in a checked out repository, explicitly declared
// in a file or not e.g., generated and updated by a .flux.yaml file, explicit Kubernetes .yaml manifests files ...
type Store interface {
	// Set the container image of a resource in the store
	SetWorkloadContainerImage(ctx context.Context, resourceID resource.ID, container string, newImageID image.Ref) error
	// UpdateWorkloadPolicies modifies a resource in the store to apply the policy-update specified.
	// It returns whether a change in the resource was actually made as a result of the change
	UpdateWorkloadPolicies(ctx context.Context, resourceID resource.ID, update resource.PolicyUpdate) (bool, error)
	// Load all the resources in the store. The returned map is indexed by the resource IDs
	GetAllResourcesByID(ctx context.Context) (map[string]resource.Resource, error)
}
