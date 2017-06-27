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

func (rc *ReleaseContext) WriteUpdates(updates []*update.ServiceUpdate) error {
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
// files and the running platform.
//
// `ServiceFilter`s can be provided to filter the found services.
// Be careful about the ordering of the filters. Filters that are earlier
// in the slice will have higher priority (they are run first).
func (rc *ReleaseContext) SelectServices(results update.Result, filters ...update.ServiceFilter) ([]*update.ServiceUpdate, error) {
	defined, err := rc.FindDefinedServices()
	if err != nil {
		return nil, err
	}

	var ids []flux.ServiceID
	definedMap := map[flux.ServiceID]*update.ServiceUpdate{}
	for _, s := range defined {
		ids = append(ids, s.ServiceID)
		definedMap[s.ServiceID] = s
	}

	// Correlate with services in running system.
	services, err := rc.cluster.SomeServices(ids)
	if err != nil {
		return nil, err
	}

	// Compare defined vs running
	var updates []*update.ServiceUpdate
	for _, s := range services {
		update, ok := definedMap[s.ID]
		if !ok {
			// Found running service, but not defined...
			continue
		}
		update.Service = s
		updates = append(updates, update)
		delete(definedMap, s.ID)
	}

	// Filter both updates ...
	var filteredUpdates []*update.ServiceUpdate
	for _, s := range updates {
		fr := s.Filter(filters...)
		results[s.ServiceID] = fr
		if fr.Status == update.ReleaseStatusSuccess || fr.Status == "" {
			filteredUpdates = append(filteredUpdates, s)
		}
	}

	// ... and missing services
	filteredDefined := map[flux.ServiceID]*update.ServiceUpdate{}
	for k, s := range definedMap {
		fr := s.Filter(filters...)
		results[s.ServiceID] = fr
		if fr.Status != update.ReleaseStatusIgnored {
			filteredDefined[k] = s
		}
	}

	// Mark anything left over as skipped
	for id, _ := range filteredDefined {
		results[id] = update.ServiceResult{
			Status: update.ReleaseStatusSkipped,
			Error:  update.NotInCluster,
		}
	}
	return filteredUpdates, nil
}

func (rc *ReleaseContext) FindDefinedServices() ([]*update.ServiceUpdate, error) {
	rc.repo.RLock()
	defer rc.repo.RUnlock()
	services, err := rc.manifests.FindDefinedServices(rc.repo.ManifestDir())
	if err != nil {
		return nil, err
	}

	var defined []*update.ServiceUpdate
	for id, paths := range services {
		switch len(paths) {
		case 1:
			def, err := ioutil.ReadFile(paths[0])
			if err != nil {
				return nil, err
			}
			defined = append(defined, &update.ServiceUpdate{
				ServiceID:     id,
				ManifestPath:  paths[0],
				ManifestBytes: def,
			})
		default:
			return nil, fmt.Errorf("multiple resource files found for service %s: %s", id, strings.Join(paths, ", "))
		}
	}
	return defined, nil
}

// Shortcut for this
func (rc *ReleaseContext) ServicesWithPolicy(p policy.Policy) (policy.ServiceMap, error) {
	rc.repo.RLock()
	defer rc.repo.RUnlock()
	return rc.manifests.ServicesWithPolicy(rc.repo.ManifestDir(), p)
}
