package sync

import (
	"github.com/pkg/errors"

	"github.com/weaveworks/flux/platform"
	"github.com/weaveworks/flux/release"
)

func Sync(rc *release.ReleaseContext) error {
	// TODO logging, metrics?
	// Get a map of resources defined in the repo
	repoResources, err := rc.Cluster.LoadManifests(rc.ManifestDir())
	if err != nil {
		return errors.Wrap(err, "loading resources from repo")
	}

	// Get a map of resources defined in the cluster
	clusterBytes, err := rc.Cluster.Export()
	if err != nil {
		return errors.Wrap(err, "exporting resource defs from cluster")
	}
	clusterResources, err := rc.Cluster.ParseManifests(clusterBytes)
	if err != nil {
		return errors.Wrap(err, "parsing exported resources")
	}

	// Everything that's in the cluster but not in the repo, delete;
	// everything that's in the repo, apply. This is an approximation
	// to figuring out what's changed, and applying that. We're
	// relying on Kubernetes to decide for each application is it is a
	// no-op.
	var sync platform.SyncDef
	for id, res := range clusterResources {
		if _, ok := repoResources[id]; !ok {
			sync.Actions = append(sync.Actions, platform.SyncAction{
				ResourceID: id,
				Delete:     res.Bytes(),
			})
		}
	}
	for id, res := range repoResources {
		sync.Actions = append(sync.Actions, platform.SyncAction{
			ResourceID: id,
			Apply:      res.Bytes(),
		})
	}

	// TODO log something?
	// TODO Record event with results?
	// TODO Notification?

	return rc.Cluster.Sync(sync)
}
