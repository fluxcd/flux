package release

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/policy"
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

func (rc *ReleaseContext) Manifests() cluster.Manifests {
	return rc.manifests
}

func (rc *ReleaseContext) WriteUpdates(updates []*update.ControllerUpdate) error {
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
	fmt.Println("\n\t\t\t================== context.SelectServices")
	fmt.Printf("\t\t\t\t----rc.Manifests: [ %+v ]\n----\n", rc.Manifests())
	fmt.Printf("\t\t\t\t----results: [ %+v ]\n----\n", results)

	// Start with all the controllers that are defined in the repo.
	allDefined, err := rc.WorkloadsForUpdate()
	fmt.Printf("\t\t\t\t----allDefined: [ %+v ]\n----\n", allDefined)
	fmt.Printf("\t\t\t\t----err: [ %+v ]\n----\n", err)
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
	fmt.Printf("\t\t\t\t----definedAndRunning: %+v\n----\n", definedAndRunning)
	fmt.Printf("\t\t\t\t----err: [ %+v ]\n----\n", err)
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

	fmt.Printf("\t\t\t\t----filteredUpdates: [ %+v ]\n----\n\n", filteredUpdates)

	return filteredUpdates, nil
}

func (rc *ReleaseContext) WorkloadsForUpdate() (map[flux.ResourceID]*update.ControllerUpdate, error) {
	resources, err := rc.manifests.LoadManifests(rc.repo.Dir(), rc.repo.ManifestDir())
	for k, v := range resources {
		fmt.Printf("\t\t\tWorkloadsForUpdate: resourceID: %s ... \n\n-----\nupdate.ControllerUpdate%+v\n-----\n\n", k, v)
	}
	fmt.Printf("\t\t\tWorkloadsForUpdate: err ... %+v\n", err)
	if err != nil {
		return nil, err
	}

	var defined = map[flux.ResourceID]*update.ControllerUpdate{}
	for _, res := range resources {
		if wl, ok := res.(resource.Workload); ok {
			defined[res.ResourceID()] = &update.ControllerUpdate{
				ResourceID:    res.ResourceID(),
				Resource:      wl,
				ManifestPath:  filepath.Join(rc.repo.Dir(), res.Source()),
				ManifestBytes: res.Bytes(),
			}
		}
	}

	fmt.Printf("\t\t\tWorkloadsForUpdate: defined ... %+v\n", defined)

	return defined, nil
}

// Shortcut for this
func (rc *ReleaseContext) ServicesWithPolicies() (policy.ResourceMap, error) {
	return rc.manifests.ServicesWithPolicies(rc.repo.ManifestDir())
}
