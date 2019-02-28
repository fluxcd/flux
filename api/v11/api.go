// This package defines the types for Flux API version 11.
package v11

import (
	"context"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/api/v10"
	"github.com/weaveworks/flux/api/v6"
)

type ListWorkloadsOptions struct {
	Namespace string
	Workloads []flux.ResourceID
}

type Server interface {
	v10.Server

	ListWorkloadsWithOptions(ctx context.Context, opts ListWorkloadsOptions) ([]v6.WorkloadStatus, error)
}

type Upstream interface {
	v10.Upstream
}
