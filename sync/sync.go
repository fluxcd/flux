package sync

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux/cluster"
	kresource "github.com/weaveworks/flux/cluster/kubernetes/resource"
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

func garbageCollect(orphans []string, clusterResources map[string]resource.Resource, clus cluster.Cluster, logger log.Logger) error {
	garbageCollect := cluster.SyncDef{}
	emptyResources := map[string]resource.Resource{"noop/noop:noop": nil}
	for _, id := range orphans {
		res, ok := clusterResources[id]
		if !ok {
			return errors.Errorf("invariant: unable to find resource %s\n", id)
		}
		if prepareSyncDelete(logger, emptyResources, id, res, &garbageCollect) {
			// TODO: use logger
			fmt.Printf("[stack-tracking] marking resource %s for deletion\n", id)
		}
	}
	return clus.Sync(garbageCollect, map[string]policy.Update{}, map[string]policy.Update{})
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

	var stackName, stackChecksum string
	resourceLabels := map[string]policy.Update{}
	resourcePolicyUpdates := map[string]policy.Update{}
	if tracks {
		stackName = "default" // TODO: multiple stack support
		stackChecksum = getStackChecksum(repoResources)

		fmt.Printf("[stack-tracking] stack=%s, checksum=%s\n", stackName, stackChecksum)

		for id := range repoResources {
			fmt.Printf("[stack-tracking] resource=%s, applying checksum=%s\n", id, stackChecksum)
			resourceLabels[id] = policy.Update{
				Add: policy.Set{"stack": stackName},
			}
			resourcePolicyUpdates[id] = policy.Update{
				Add: policy.Set{policy.StackChecksum: stackChecksum},
			}
		}
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

	if err := clus.Sync(sync, resourceLabels, resourcePolicyUpdates); err != nil {
		return err
	}
	if tracks {
		orphanedResources := make([]string, 0)

		fmt.Printf("[stack-tracking] scanning stack (%s) for orphaned resources\n", stackName)
		clusterResourceBytes, err := clus.ExportByLabel(fmt.Sprintf("%s%s", kresource.PolicyPrefix, "stack"), stackName)
		if err != nil {
			return errors.Wrap(err, "exporting resource defs from cluster post-sync")
		}
		clusterResources, err = m.ParseManifests(clusterResourceBytes)
		if err != nil {
			return errors.Wrap(err, "parsing exported resources post-sync")
		}

		for resourceID, res := range clusterResources {
			if res.Policy().Has(policy.StackChecksum) {
				val, _ := res.Policy().Get(policy.StackChecksum)
				if val != stackChecksum {
					fmt.Printf("[stack-tracking] cluster resource=%s, invalid checksum=%s\n", resourceID, val)
					orphanedResources = append(orphanedResources, resourceID)
				} else {
					fmt.Printf("[stack-tracking] cluster resource ok: %s\n", resourceID)
				}
			} else {
				fmt.Printf("warning: [stack-tracking] cluster resource=%s, missing policy=%s\n", resourceID, policy.StackChecksum)
			}
		}

		if len(orphanedResources) > 0 {
			return garbageCollect(orphanedResources, clusterResources, clus, logger)
		}
	}

	return nil
}

func prepareSyncDelete(logger log.Logger, repoResources map[string]resource.Resource, id string, res resource.Resource, sync *cluster.SyncDef) bool {
	if len(repoResources) == 0 {
		return false
	}
	if res.Policy().Has(policy.Ignore) {
		logger.Log("resource", res.ResourceID(), "ignore", "delete")
		return false
	}
	if _, ok := repoResources[id]; !ok {
		sync.Actions = append(sync.Actions, cluster.SyncAction{
			Delete: res,
		})
		return true
	}
	return false
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
