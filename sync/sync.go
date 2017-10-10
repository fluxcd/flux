package sync

import (
	"fmt"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
)

// Sync synchronises the cluster to the files in a directory
func Sync(m cluster.Manifests, repoResources map[string]resource.Resource, clus cluster.Cluster, deletes bool, logger log.Logger) error {
	// Get a map of resources defined in the cluster
	clusterBytes, err := clus.Export()
	if err != nil {
		return errors.Wrap(err, "exporting resource defs from cluster")
	}
	clusterResources, err := m.ParseManifests(clusterBytes)
	if err != nil {
		return errors.Wrap(err, "parsing exported resources")
	}

	// Everything that's in the cluster but not in the repo, delete;
	// everything that's in the repo, apply. This is an approximation
	// to figuring out what's changed, and applying that. We're
	// relying on Kubernetes to decide for each application if it is a
	// no-op.
	sync := cluster.SyncDef{}

	nsRepoResources := make(map[string]resource.Resource)
	otherRepoResources := make(map[string]resource.Resource)
	for id, res := range repoResources {
		_, kind, name := res.ResourceID().Components()
		fmt.Printf("\n---------SPLIT----------\n<<< kind: %s, name: %s\n-------------------\n, ", kind, name)
		if kind == "namespace" {
			nsRepoResources[id] = res
		} else {
			otherRepoResources[id] = res
		}
	}

	//fmt.Printf("\n~~~ nsRepoResources: %+v, otherRepoResources: %+v\n\n", nsRepoResources, otherRepoResources)

	// First tackle resources that are not Namespace kind, in case we are deleting the Namespace as well
	// Deleting a Namespace first, then a resource in this namespace causes an error
	deletes=true
	if deletes {
		for id, res := range otherRepoResources {
			prepareSyncDelete(logger, repoResources, id, res, &sync)
		}
		for id, res := range nsRepoResources {
			prepareSyncDelete(logger, repoResources, id, res, &sync)
		}
	}

	// To avoid errors due to a non existent namespace if a resource in that namespace is created first,
	// create Namespace objects first
	for id, res := range nsRepoResources {
		prepareSyncApply(logger, clusterResources, id, res, &sync)
	}
	for id, res := range otherRepoResources {
		prepareSyncApply(logger, clusterResources, id, res, &sync)
	}

	//fmt.Printf("\n!!! sync actions: %#v\n\n", sync.Actions)
	return clus.Sync(sync)
}

func prepareSyncDelete(logger log.Logger, repoResources map[string]resource.Resource, id string, res resource.Resource, sync *cluster.SyncDef) {
	if res.Policy().Contains(policy.Ignore) {
		logger.Log("resource", res.ResourceID(), "ignore", "delete")
		return
	}
	if _, ok := repoResources[id]; !ok {
		ns, k, n := res.ResourceID().Components()
		fmt.Printf("\n-----------DELETE--------\n>>> res: %s, %s, %s\n-------------------\n, ", ns, k, n)
		sync.Actions = append(sync.Actions, cluster.SyncAction{
			ResourceID: id,
			Delete:     res.Bytes(),
		})
	}
}

func prepareSyncApply(logger log.Logger, clusterResources map[string]resource.Resource, id string, res resource.Resource, sync *cluster.SyncDef) {
	if res.Policy().Contains(policy.Ignore) {
		logger.Log("resource", res.ResourceID(), "ignore", "apply")
		return
	}
	if cres, ok := clusterResources[id]; ok {
		if cres.Policy().Contains(policy.Ignore) {
			logger.Log("resource", res.ResourceID(), "ignore", "apply")
			return
		}
	}

	ns, k, n := res.ResourceID().Components()
	fmt.Printf("\n---------APPLY----------\n>>> res: %s, %s, %s\n-------------------\n, ", ns, k, n)
	sync.Actions = append(sync.Actions, cluster.SyncAction{
		ResourceID: id,
		Apply:      res.Bytes(),
	})
}
