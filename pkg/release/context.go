package release

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/fluxcd/flux/pkg/cluster"
	"github.com/fluxcd/flux/pkg/manifests"
	"github.com/fluxcd/flux/pkg/registry"
	"github.com/fluxcd/flux/pkg/resource"
	"github.com/fluxcd/flux/pkg/update"
)

type ReleaseContext struct {
	cluster       cluster.Cluster
	resourceStore manifests.Store
	registry      registry.Registry
}

func NewReleaseContext(cluster cluster.Cluster, resourceStore manifests.Store, registry registry.Registry) *ReleaseContext {
	return &ReleaseContext{
		cluster:       cluster,
		resourceStore: resourceStore,
		registry:      registry,
	}
}

func (rc *ReleaseContext) Registry() registry.Registry {
	return rc.registry
}

func (rc *ReleaseContext) GetAllResources(ctx context.Context) (map[string]resource.Resource, error) {
	return rc.resourceStore.GetAllResourcesByID(ctx)
}

func (rc *ReleaseContext) WriteUpdates(ctx context.Context, updates []*update.WorkloadUpdate) error {
	err := func() error {
		for _, update := range updates {
			for _, container := range update.Updates {
				err := rc.resourceStore.SetWorkloadContainerImage(ctx, update.ResourceID, container.Container, container.Target)
				if err != nil {
					return errors.Wrapf(err, "updating resource %s in %s", update.ResourceID.String(), update.Resource.Source())
				}
			}
		}
		return nil
	}()
	return err
}

// ---

// SelectWorkloads finds the workloads that exist both in the definition
// files and the running cluster. `WorkloadFilter`s can be provided
// to filter the controllers so found, either before (`prefilters`) or
// after (`postfilters`) consulting the cluster.
func (rc *ReleaseContext) SelectWorkloads(ctx context.Context, results update.Result, prefilters,
	postfilters []update.WorkloadFilter) ([]*update.WorkloadUpdate, error) {

	// Start with all the workloads that are defined in the repo.
	allDefined, err := rc.WorkloadsForUpdate(ctx)
	if err != nil {
		return nil, err
	}

	// Apply prefilters to select the controllers that we'll ask the
	// cluster about.
	var toAskClusterAbout []resource.ID
	for _, s := range allDefined {
		res := s.Filter(prefilters...)
		if res.Error == "" {
			// Give these a default value, in case we cannot access them
			// in the cluster.
			results[s.ResourceID] = update.WorkloadResult{
				Status: update.ReleaseStatusSkipped,
				Error:  update.NotAccessibleInCluster,
			}
			toAskClusterAbout = append(toAskClusterAbout, s.ResourceID)
		} else {
			results[s.ResourceID] = res
		}
	}

	// Ask the cluster about those that we're still interested in
	definedAndRunning, err := rc.cluster.SomeWorkloads(ctx, toAskClusterAbout)
	if err != nil {
		return nil, err
	}

	var forPostFiltering []*update.WorkloadUpdate
	// Compare defined vs running
	for _, s := range definedAndRunning {
		update, ok := allDefined[s.ID]
		if !ok {
			// A contradiction: we asked only about defined
			// workloads, and got a workload that is not
			// defined.
			return nil, fmt.Errorf("workload %s was requested and is running, but is not defined", s.ID)
		}
		update.Workload = s
		forPostFiltering = append(forPostFiltering, update)
	}

	var filteredUpdates []*update.WorkloadUpdate
	for _, s := range forPostFiltering {
		fr := s.Filter(postfilters...)
		results[s.ResourceID] = fr
		if fr.Status == update.ReleaseStatusSuccess || fr.Status == "" {
			filteredUpdates = append(filteredUpdates, s)
		}
	}

	return filteredUpdates, nil
}

// WorkloadsForUpdate collects all workloads defined in manifests and prepares a list of
// workload updates for each of them.  It does not consider updatability.
func (rc *ReleaseContext) WorkloadsForUpdate(ctx context.Context) (map[resource.ID]*update.WorkloadUpdate, error) {
	resources, err := rc.GetAllResources(ctx)
	if err != nil {
		return nil, err
	}

	var defined = map[resource.ID]*update.WorkloadUpdate{}
	for _, res := range resources {
		if wl, ok := res.(resource.Workload); ok {
			defined[res.ResourceID()] = &update.WorkloadUpdate{
				ResourceID: res.ResourceID(),
				Resource:   wl,
			}
		}
	}
	return defined, nil
}
