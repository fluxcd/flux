package sync

import (
	"github.com/pkg/errors"

	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/release"
	"github.com/weaveworks/flux/resource"
)

const (
	ResultDelete = "delete"
	ResultApply  = "apply"

	IgnoreAnnotation = "flux.weave.works/ignore"
)

type Result struct {
	Rev     string
	Actions map[string]string
}

// Parameters for sync; packaged into a struct mainly so that it's
// convenient to change the parameters without revisiting all the RPC
// boilerplate.
type Params struct {
	DryRun  bool
	Deletes bool
}

// Extract the result from a compilation of actions.
// TODO consider how this and the cluster.SyncDef types can be
// merged.
func ExtractResult(rev string, def cluster.SyncDef) *Result {
	result := &Result{
		Rev:     rev,
		Actions: map[string]string{},
	}
	for _, action := range def.Actions {
		if action.Delete != nil {
			result.Actions[action.ResourceID] = ResultDelete
		}
		// NB Apply implicitly overrides Delete here, on the grounds
		// that deleting-then-applying ~= applying
		if action.Apply != nil {
			result.Actions[action.ResourceID] = ResultApply
		}
	}
	return result
}

func Sync(rc *release.ReleaseContext, deletes bool, dryRun bool) (*Result, error) {
	// TODO logging, metrics?
	// Get a map of resources defined in the repo

	rev, err := rc.HeadRevision()
	if err != nil {
		return nil, errors.Wrap(err, "getting revision of repo")
	}

	repoResources, err := rc.Cluster.LoadManifests(rc.ManifestDir())
	if err != nil {
		return nil, errors.Wrap(err, "loading resources from repo")
	}

	// Get a map of resources defined in the cluster
	clusterBytes, err := rc.Cluster.Export()
	if err != nil {
		return nil, errors.Wrap(err, "exporting resource defs from cluster")
	}
	clusterResources, err := rc.Cluster.ParseManifests(clusterBytes)
	if err != nil {
		return nil, errors.Wrap(err, "parsing exported resources")
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

	// TODO log something?
	// TODO Record event with results?
	// TODO Notification?

	result := ExtractResult(rev, sync)
	if dryRun {
		return result, nil
	}

	err = rc.Cluster.Sync(sync)
	if err == nil {
		err = rc.UpdateTag()
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

func ignore(res resource.Resource) bool {
	notes := res.Annotations()
	if notes == nil {
		return false
	}
	return notes[IgnoreAnnotation] == "true"
}
