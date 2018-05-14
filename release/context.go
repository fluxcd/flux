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
	fmt.Println("\n\t\t\t================== in context.SelectServices")
	fmt.Printf("\t\t\t\t---- 1 results: [ %+v ] \n", results)
	fmt.Printf("\t\t\t\t---- 1 prefilters: [ %+v ] \n", prefilters)
	fmt.Printf("\t\t\t\t---- 1 postfilters: [ %+v ] \n", postfilters)

	// Start with all the controllers that are defined in the repo.
	allDefined, err := rc.WorkloadsForUpdate()
	if err != nil {
		return nil, err
	}

	fmt.Println("\tvvvv--------- allDefined (workloads for upgrade)")
	for k, v := range allDefined {
		fmt.Printf("\t\t\t\t*** ResourceID: %v [ %+v ]\n----\n", k, v.ResourceID)
		fmt.Printf("\t\t\t\t*** Policy: %v [ %+v ]\n----\n", k, v.Resource.Policy())
		fmt.Printf("\t\t\t\t*** Containers: %v [ %+v ]\n----\n", k, v.Resource.Containers())
	}
	fmt.Println("\t^^^^--------- allDefined ----------------")

	// Apply prefilters to select the controllers that we'll ask the
	// cluster about.
	var toAskClusterAbout []flux.ResourceID

	fmt.Println("Loop through allDefined ...")
	for _, s := range allDefined {
		res := s.Filter(prefilters...)

		fmt.Printf("\t*** : ResourceId = %v [ Status: %+v ][ Error: %+v ]\n----\n", s.ResourceID, res.Status, res.Error)

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

	fmt.Println("\tvvvv--------- results")
	for k, v := range results {
		fmt.Printf("\t\t\t\tResourceID: %v [ %+v ]\n----\n", k, v.Status)
		fmt.Printf("\t\t\t\tPolicy: %v [ %+v ]\n----\n", k, v.PerContainer)
	}
	fmt.Println("\t^^^^--------- allDefined")

	// Ask the cluster about those that we're still interested in
	definedAndRunning, err := rc.cluster.SomeControllers(toAskClusterAbout)
	if err != nil {
		return nil, err
	}

	var forPostFiltering []*update.ControllerUpdate
	// Compare defined vs running
	fmt.Println("\tvvvv----------- definedAndRunning")
	for _, s := range definedAndRunning {
		fmt.Printf("\t\t\t\tID: %+v\n", s.ID)
		fmt.Printf("\t\t\t\tContainers: %+v\n", s.Containers)

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
	fmt.Println("\t^^^^----------- definedAndRunning")

	var filteredUpdates []*update.ControllerUpdate
	for _, s := range forPostFiltering {
		fr := s.Filter(postfilters...)
		results[s.ResourceID] = fr
		if fr.Status == update.ReleaseStatusSuccess || fr.Status == "" {
			filteredUpdates = append(filteredUpdates, s)
		}
	}
	fmt.Printf("\t\t\t\t---- 3 results: [ %+v ] \n", results)

	fmt.Println("\t\t\t\t---- definedAndRunning")
	for _, v := range definedAndRunning {
		fmt.Printf("\t\t\t\tID: [ %+v ]\n----\n", v.ID)
		fmt.Printf("\t\t\t\tContainers: [ %+v ]\n----\n", v.Containers)
	}

	fmt.Println("\n\t\t\t================== END of context.SelectServices")

	return filteredUpdates, nil
}

func (rc *ReleaseContext) WorkloadsForUpdate() (map[flux.ResourceID]*update.ControllerUpdate, error) {
	resources, err := rc.manifests.LoadManifests(rc.repo.Dir(), rc.repo.ManifestDir())

	fmt.Printf("\t\t\t*** In WorkloadsForUpdate: err after getting resources ... %+v, len of resources: %d\n", err, len(resources))
	if err != nil {
		return nil, err
	}

	var defined = map[flux.ResourceID]*update.ControllerUpdate{}
	// PROBLEM IS HERE - DEBUG
	for _, res := range resources {
		if wl, ok := res.(resource.Workload); ok {
			fmt.Printf("\t\t\t---> ResourceID %v\n", wl.ResourceID())
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
