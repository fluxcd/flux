package sync

import (
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
)

// Synchronise the cluster to the files in a directory
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
	var sync cluster.SyncDef

	if deletes {
		for id, res := range clusterResources {
			if res.Policy().Contains(policy.Ignore) {
				logger.Log("resource", res.ResourceID(), "ignore", "delete")
				continue
			}
			if _, ok := repoResources[id]; !ok {
				sync.Actions = append(sync.Actions, cluster.SyncAction{
					ResourceID: id,
					Delete:     res.Bytes(),
				})
			}
		}
	}

	for id, res := range repoResources {
		if res.Policy().Contains(policy.Ignore) {
			logger.Log("resource", res.ResourceID(), "ignore", "apply")
			continue
		}
		if cres, ok := clusterResources[id]; ok {
			if cres.Policy().Contains(policy.Ignore) {
				logger.Log("resource", res.ResourceID(), "ignore", "apply")
				continue
			}
		}
		sync.Actions = append(sync.Actions, cluster.SyncAction{
			ResourceID: id,
			Apply:      res.Bytes(),
		})
	}
	return clus.Sync(sync)
}
