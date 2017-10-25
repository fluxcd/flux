package sync

import (
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

	nsClusterResources, otherClusterResources := separateResourcesByType(clusterResources)
	nsRepoResources, otherRepoResources := separateResourcesByType(repoResources)

	// First tackle resources that are not Namespace kind, in case we are deleting the Namespace as well
	// Deleting a Namespace first, then a resource in this namespace causes an error

	// DANGER ZONE (tamara) This works and is dangerous. At the moment will delete Flux and
	// other pods unless the relevant manifests are part of the user repo. Needs a lot of thought
	// before this cleanup cluster feature can be unleashed on the world.
	if deletes {
		for id, res := range otherClusterResources {
			prepareSyncDelete(logger, repoResources, id, res, &sync)
		}
		for id, res := range nsClusterResources {
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

	return clus.Sync(sync)
}

func separateResourcesByType(resources map[string]resource.Resource) (map[string]resource.Resource, map[string]resource.Resource) {
	if len(resources) == 0 {
		return nil, nil
	}
	nsResources := make(map[string]resource.Resource)
	otherResources := make(map[string]resource.Resource)
	for id, res := range resources {
		_, kind, _ := res.ResourceID().Components()
		if kind == "namespace" {
			nsResources[id] = res
		} else {
			otherResources[id] = res
		}
	}
	return nsResources, otherResources
}

func prepareSyncDelete(logger log.Logger, repoResources map[string]resource.Resource, id string, res resource.Resource, sync *cluster.SyncDef) {
	if len(repoResources) == 0 {
		return
	}
	if res.Policy().Contains(policy.Ignore) {
		logger.Log("resource", res.ResourceID(), "ignore", "delete")
		return
	}
	if _, ok := repoResources[id]; !ok {
		sync.Actions = append(sync.Actions, cluster.SyncAction{
			ResourceID: id,
			Delete:     res.Bytes(),
		})
	}
}

func prepareSyncApply(logger log.Logger, clusterResources map[string]resource.Resource, id string, res resource.Resource, sync *cluster.SyncDef) {
	if len(clusterResources) == 0 {
		return
	}

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
	sync.Actions = append(sync.Actions, cluster.SyncAction{
		ResourceID: id,
		Apply:      res.Bytes(),
	})
}
