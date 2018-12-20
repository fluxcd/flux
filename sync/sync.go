package sync

import (
	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
)

// Syncer has the methods we need to be able to compile and run a sync
type Syncer interface {
	Sync(cluster.SyncDef) error
}

// Sync synchronises the cluster to the files under a directory.
func Sync(logger log.Logger, repoResources map[string]resource.Resource, clus Syncer) error {
	// TODO: multiple stack support. This will involve partitioning
	// the resources into disjoint maps, then passing each to
	// makeStack.
	defaultStack := makeStack("default", repoResources, logger)

	sync := cluster.SyncDef{Stacks: []cluster.SyncStack{defaultStack}}
	if err := clus.Sync(sync); err != nil {
		return err
	}
	return nil
}

func makeStack(name string, repoResources map[string]resource.Resource, logger log.Logger) cluster.SyncStack {
	stack := cluster.SyncStack{Name: name}
	var resources []resource.Resource
	for _, res := range repoResources {
		resources = append(resources, res)
		if res.Policy().Has(policy.Ignore) {
			logger.Log("resource", res.ResourceID(), "ignore", "apply")
			continue
		}
	}

	stack.Resources = resources
	return stack
}
