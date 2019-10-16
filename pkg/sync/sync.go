package sync

import (
	"github.com/fluxcd/flux/pkg/cluster"
	"github.com/fluxcd/flux/pkg/resource"
)

// Syncer has the methods we need to be able to compile and run a sync
type Syncer interface {
	Sync(cluster.SyncSet) error
}

// Sync synchronises the cluster to the files under a directory.
func Sync(setName string, repoResources map[string]resource.Resource, clus Syncer) error {
	set := makeSet(setName, repoResources)
	if err := clus.Sync(set); err != nil {
		return err
	}
	return nil
}

func makeSet(name string, repoResources map[string]resource.Resource) cluster.SyncSet {
	s := cluster.SyncSet{Name: name}
	var resources []resource.Resource
	for _, res := range repoResources {
		resources = append(resources, res)
	}
	s.Resources = resources
	return s
}
