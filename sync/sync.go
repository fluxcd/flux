package sync

import (
	"github.com/pkg/errors"

	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/resource"
)

const (
	ResultDelete = "delete"
	ResultApply  = "apply"

	IgnoreAnnotation = "flux.weave.works/ignore"
)

// Synchronise the cluster to the files in a directory
func Sync(manifestDir string, clus cluster.Cluster, deletes bool) error {
	// TODO logging, metrics?
	// Get a map of resources defined in the repo
	repoResources, err := clus.LoadManifests(manifestDir)
	if err != nil {
		return errors.Wrap(err, "loading resources from repo")
	}

	// Get a map of resources defined in the cluster
	clusterBytes, err := clus.Export()
	if err != nil {
		return errors.Wrap(err, "exporting resource defs from cluster")
	}
	clusterResources, err := clus.ParseManifests(clusterBytes)
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
			if ignore(res) {
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
		if ignore(res) {
			continue
		}
		if cres, ok := clusterResources[id]; ok {
			if ignore(cres) {
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

func ignore(res resource.Resource) bool {
	notes := res.Annotations()
	if notes == nil {
		return false
	}
	return notes[IgnoreAnnotation] == "true"
}
