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

// Sync synchronises the cluster to the files under a directory.
func Sync(logger log.Logger, m cluster.Manifests, repoResources map[string]resource.Resource, clus cluster.Cluster) error {
	// Get a map of resources defined in the cluster
	clusterBytes, err := clus.Export()

	if err != nil {
		return errors.Wrap(err, "exporting resource defs from cluster")
	}
	clusterResources, err := m.ParseManifests(clusterBytes)
	if err != nil {
		return errors.Wrap(err, "parsing exported resources")
	}

	// TODO: multiple stack support. This will involve partitioning
	// the resources into disjoint maps, then passing each to
	// makeStack.
	defaultStack := makeStack("default", repoResources, clusterResources, logger)

	sync := cluster.SyncDef{Stacks: []cluster.SyncStack{defaultStack}}
	if err := clus.Sync(sync); err != nil {
		return err
	}
	return nil
}

func makeStack(name string, repoResources, clusterResources map[string]resource.Resource, logger log.Logger) cluster.SyncStack {
	stack := cluster.SyncStack{Name: name}
	var resources []resource.Resource

	// To get a stable checksum, we have to sort the resources.
	var ids []string
	for id, _ := range repoResources {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	checksum := sha1.New()
	for _, id := range ids {
		res := repoResources[id]
		if res.Policy().Has(policy.Ignore) {
			logger.Log("resource", res.ResourceID(), "ignore", "apply")
			continue
		}
		// It may be ignored in the cluster, but it isn't in the repo;
		// and we don't want what happens in the cluster to affect the
		// checksum.
		checksum.Write(res.Bytes())

		if cres, ok := clusterResources[id]; ok {
			if cres.Policy().Has(policy.Ignore) {
				logger.Log("resource", res.ResourceID(), "ignore", "apply")
				continue
			}
		}
		resources = append(resources, res)
	}

	stack.Resources = resources
	stack.Checksum = hex.EncodeToString(checksum.Sum(nil))
	return stack
}
