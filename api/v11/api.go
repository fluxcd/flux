// This package defines the types for Flux API version 11.
package v11

import (
	"context"

	"github.com/weaveworks/flux/api/v10"
	"github.com/weaveworks/flux/api/v6"
	"github.com/weaveworks/flux/resource"
)

type ListServicesOptions struct {
	Namespace string
	Services  []resource.ID
}

type Server interface {
	v10.Server

	ListServicesWithOptions(ctx context.Context, opts ListServicesOptions) ([]v6.ControllerStatus, error)
}

type Upstream interface {
	v10.Upstream
}
