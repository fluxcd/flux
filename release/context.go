package release

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/registry"
	"github.com/weaveworks/flux/resource"
	"github.com/weaveworks/flux/update"
)

type ReleaseContext struct {
	cluster   cluster.Cluster
	manifests cluster.Manifests
	repo      *git.Checkout
	registry  registry.Registry
}

func NewReleaseContext(c cluster.Cluster, m cluster.Manifests, reg registry.Registry, repo *git.Checkout) *ReleaseContext {
	return &ReleaseContext{
		cluster:   c,
		manifests: m,
		repo:      repo,
		registry:  reg,
	}
}

func (rc *ReleaseContext) Registry() registry.Registry {
	return rc.registry
}

func (rc *ReleaseContext) LoadManifests() (map[string]resource.Resource, error) {
	return rc.manifests.LoadManifests(rc.repo.Dir(), rc.repo.ManifestDirs())
}

func (rc *ReleaseContext) WriteUpdates(updates []*update.ControllerUpdate) error {
	err := func() error {
		for _, update := range updates {
			manifestBytes, err := ioutil.ReadFile(update.ManifestPath)
			if err != nil {
				return err
			}
			for _, container := range update.Updates {
				manifestBytes, err = rc.manifests.UpdateImage(manifestBytes, update.ResourceID, container.Container, container.Target)
				if err != nil {
					return err
				}
			}
			if err = ioutil.WriteFile(update.ManifestPath, manifestBytes, os.FileMode(0600)); err != nil {
				return err
			}
		}
		return nil
	}()
	return err
}

// ---

// SelectServices finds the services that exist both in the definition
// files and the running cluster. `ControllerFilter`s can be provided
// to filter the controllers so found, either before (`prefilters`) or
// after (`postfilters`) consulting the cluster.
func (rc *ReleaseContext) SelectServices(results update.Result, prefilters, postfilters []update.ControllerFilter) ([]*update.ControllerUpdate, error) {

	// Start with all the controllers that are defined in the repo.
	allDefined, err := rc.WorkloadsForUpdate()
	if err != nil {
		return nil, err
	}

	// Apply prefilters to select the controllers that we'll ask the
	// cluster about.
	var toAskClusterAbout []flux.ResourceID
	for _, s := range allDefined {
		res := s.Filter(prefilters...)
		if res.Error == "" {
			// Give these a default value, in case we don't find them
			// in the cluster.
			results[s.ResourceID] = update.ControllerResult{
				Status: update.ReleaseStatusSkipped,
				Error:  update.NotInCluster,
			}
			toAskClusterAbout = append(toAskClusterAbout, s.ResourceID)
		} else {
			results[s.ResourceID] = res
		}
	}

	// Ask the cluster about those that we're still interested in
	definedAndRunning, err := rc.cluster.SomeControllers(toAskClusterAbout)
	if err != nil {
		return nil, err
	}

	var forPostFiltering []*update.ControllerUpdate
	// Compare defined vs running
	for _, s := range definedAndRunning {
		update, ok := allDefined[s.ID]
		if !ok {
			// A contradiction: we asked only about defined
			// controllers, and got a controller that is not
			// defined.
			return nil, fmt.Errorf("controller %s was requested and is running, but is not defined", s.ID)
		}
		update.Controller = s
		forPostFiltering = append(forPostFiltering, update)
	}

	var filteredUpdates []*update.ControllerUpdate
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
// controller updates for each of them.  It does not consider updatability.
func (rc *ReleaseContext) WorkloadsForUpdate() (map[flux.ResourceID]*update.ControllerUpdate, error) {
	resources, err := rc.LoadManifests()
	if err != nil {
		return nil, err
	}

	var defined = map[flux.ResourceID]*update.ControllerUpdate{}
	for _, res := range resources {
		if wl, ok := res.(resource.Workload); ok {
			defined[res.ResourceID()] = &update.ControllerUpdate{
				ResourceID:   res.ResourceID(),
				Resource:     wl,
				ManifestPath: filepath.Join(rc.repo.Dir(), res.Source()),
			}
		}
	}
	return defined, nil
}
