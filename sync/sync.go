package sync

import (
	"crypto/sha1"
	"encoding/hex"
	"sort"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
)

// Checksum generates a unique identifier for all apply actions in the stack
func getStackChecksum(repoResources map[string]resource.Resource) string {
	checksum := sha1.New()

	sortedKeys := make([]string, 0, len(repoResources))
	for resourceID := range repoResources {
		sortedKeys = append(sortedKeys, resourceID)
	}
	sort.Strings(sortedKeys)
	for resourceIDIndex := range sortedKeys {
		checksum.Write(repoResources[sortedKeys[resourceIDIndex]].Bytes())
	}
	return hex.EncodeToString(checksum.Sum(nil))
}

// Sync synchronises the cluster to the files in a directory
func Sync(logger log.Logger, m cluster.Manifests, repoResources map[string]resource.Resource, clus cluster.Cluster,
	deletes, tracks bool) error {
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

	resourceLabels := map[string]policy.Update{}
	resourcePolicyUpdates := map[string]policy.Update{}
	if tracks {
		stackName := "default" // TODO: multiple stack support
		stackChecksum := getStackChecksum(repoResources)

		logger.Log("stack", stackName, "checksum", stackChecksum)

		for id := range repoResources {
			logger.Log("resource", id, "applying checksum", stackChecksum)
			resourceLabels[id] = policy.Update{
				Add: policy.Set{"stack": stackName},
			}
			resourcePolicyUpdates[id] = policy.Update{
				Add: policy.Set{policy.StackChecksum: stackChecksum},
			}
		}

		// label flux.weave.works/stack
	}

	// DANGER ZONE (tamara) This works and is dangerous. At the moment will delete Flux and
	// other pods unless the relevant manifests are part of the user repo. Needs a lot of thought
	// before this cleanup cluster feature can be unleashed on the world.
	if deletes {
		for id, res := range clusterResources {
			prepareSyncDelete(logger, repoResources, id, res, &sync)
		}
	}

	for id, res := range repoResources {
		prepareSyncApply(logger, clusterResources, id, res, &sync)
	}

	return clus.Sync(sync, resourceLabels, resourcePolicyUpdates)
}

func prepareSyncDelete(logger log.Logger, repoResources map[string]resource.Resource, id string, res resource.Resource, sync *cluster.SyncDef) {
	if len(repoResources) == 0 {
		return
	}
	if res.Policy().Has(policy.Ignore) {
		logger.Log("resource", res.ResourceID(), "ignore", "delete")
		return
	}
	if _, ok := repoResources[id]; !ok {
		sync.Actions = append(sync.Actions, cluster.SyncAction{
			Delete: res,
		})
	}
}

func prepareSyncApply(logger log.Logger, clusterResources map[string]resource.Resource, id string, res resource.Resource, sync *cluster.SyncDef) {
	if res.Policy().Has(policy.Ignore) {
		logger.Log("resource", res.ResourceID(), "ignore", "apply")
		return
	}
	if cres, ok := clusterResources[id]; ok {
		if cres.Policy().Has(policy.Ignore) {
			logger.Log("resource", res.ResourceID(), "ignore", "apply")
			return
		}
	}
	sync.Actions = append(sync.Actions, cluster.SyncAction{
		Apply: res,
	})
}
