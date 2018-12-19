package sync

import (
	"crypto/sha1"
	"encoding/hex"
	"sort"

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

	// To get a stable checksum, we have to sort the resources.
	var ids []string
	for id, _ := range repoResources {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	checksum := sha1.New()
	for _, id := range ids {
		res := repoResources[id]
		resources = append(resources, res)
		if res.Policy().Has(policy.Ignore) {
			logger.Log("resource", res.ResourceID(), "ignore", "apply")
			continue
		}
		// Ignored resources are not included in the checksum; this
		// means if you mark something as ignored, the checksum will
		// come out differently. But the alternative is that adding
		// ignored resources changes the checksum even though they are
		// not intended to be created.
		checksum.Write(res.Bytes())
	}

	stack.Resources = resources
	stack.Checksum = hex.EncodeToString(checksum.Sum(nil))
	return stack
}
