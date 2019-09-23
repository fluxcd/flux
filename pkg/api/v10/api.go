// This package defines the types for Flux API version 10.
package v10

import (
	"context"

	"github.com/fluxcd/flux/pkg/api/v6"
	"github.com/fluxcd/flux/pkg/api/v9"
	"github.com/fluxcd/flux/pkg/update"
)

type ListImagesOptions struct {
	Spec                    update.ResourceSpec
	OverrideContainerFields []string
	Namespace               string
}

type Server interface {
	v6.NotDeprecated

	ListImagesWithOptions(ctx context.Context, opts ListImagesOptions) ([]v6.ImageStatus, error)
}

type Upstream interface {
	v9.Upstream
}
