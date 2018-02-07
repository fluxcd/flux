package release

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/registry"
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

func (rc *ReleaseContext) Manifests() cluster.Manifests {
	return rc.manifests
}

func (rc *ReleaseContext) WriteUpdates(updates []*update.ControllerUpdate) error {
	rc.repo.Lock()
	defer rc.repo.Unlock()
	err := func() error {
		for _, update := range updates {
			fi, err := os.Stat(update.ManifestPath)
			if err != nil {
				return err
			}
			if err = ioutil.WriteFile(update.ManifestPath, update.ManifestBytes, fi.Mode()); err != nil {
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
	allDefined, err := rc.FindDefinedServices()
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

func (rc *ReleaseContext) FindDefinedServices() (map[flux.ResourceID]*update.ControllerUpdate, error) {
	rc.repo.RLock()
	defer rc.repo.RUnlock()
	services, err := rc.manifests.FindDefinedServices(rc.repo.ManifestDir())
	if err != nil {
		return nil, err
	}

	var defined = map[flux.ResourceID]*update.ControllerUpdate{}
	for id, paths := range services {
		switch len(paths) {
		case 1:
			def, err := ioutil.ReadFile(paths[0])
			if err != nil {
				return nil, err
			}
			defined[id] = &update.ControllerUpdate{
				ResourceID:    id,
				ManifestPath:  paths[0],
				ManifestBytes: def,
			}
		default:
			return nil, fmt.Errorf("multiple resource files found for service %s: %s", id, strings.Join(paths, ", "))
		}
	}
	return defined, nil
}

// Shortcut for this
func (rc *ReleaseContext) ServicesWithPolicies() (policy.ResourceMap, error) {
	rc.repo.RLock()
	defer rc.repo.RUnlock()
	return rc.manifests.ServicesWithPolicies(rc.repo.ManifestDir())
}
