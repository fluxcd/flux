package daemon

import (
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"sync"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/history"
	"github.com/weaveworks/flux/resource"
	fluxsync "github.com/weaveworks/flux/sync"
	"github.com/weaveworks/flux/update"
)

// Loop for potentially long-running stuff. This includes running
// jobs, and looking for new commits.

func (d *Daemon) Loop(stop chan struct{}, wg *sync.WaitGroup, logger log.Logger) {
	defer wg.Done()
	pollGit := time.NewTimer(d.GitPollInterval)
	resetGitPoll := func() {
		if pollGit != nil {
			pollGit.Stop()
			pollGit = time.NewTimer(d.GitPollInterval)
		}
	}

	pollImages := time.Tick(d.RegistryPollInterval)
	// Ask for a sync straight away
	d.askForSync()
	for {
		select {
		case <-stop:
			logger.Log("stopping", "true")
			return
		case <-d.syncSoon:
			d.pullAndSync(logger)
			resetGitPoll()
		case <-pollGit.C:
			// Time to poll for new commits (unless we're already
			// about to do that)
			d.askForSync()
		case <-pollImages:
			// Time to poll for new images
			d.PollImages(logger)
		case job := <-d.Jobs.Ready():
			jobLogger := log.NewContext(logger).With("jobID", job.ID)
			jobLogger.Log("state", "in-progress")
			// It's assumed that (successful) jobs will push commits
			// to the upstream repo, and therefore we probably want to
			// pull from there and sync the cluster.
			if err := job.Do(jobLogger); err != nil {
				jobLogger.Log("state", "done", "success", "false", "err", err)
				continue
			}
			jobLogger.Log("state", "done", "success", "true")
			d.askForSync()
		}
	}
}

// Ask for a sync, or if there's one waiting, let that happen.
func (d *Daemon) askForSync() {
	d.initSyncSoon.Do(func() {
		d.syncSoon = make(chan struct{}, 1)
	})
	select {
	case d.syncSoon <- struct{}{}:
	default:
	}
}

func (d *Daemon) pullAndSync(logger log.Logger) {
	started := time.Now().UTC()

	// Pull for new commits
	if err := d.Checkout.Pull(); err != nil {
		logger.Log("err", err)
		return
	}

	// checkout a working clone so we can mess around with tags later
	working, err := d.Checkout.WorkingClone()
	if err != nil {
		logger.Log("err", err)
		return
	}
	defer working.Clean()

	// TODO logging, metrics?
	// Get a map of all resources defined in the repo
	allResources, err := d.Manifests.LoadManifests(working.ManifestDir())
	if err != nil {
		logger.Log("err", errors.Wrap(err, "loading resources from repo"))
		return
	}

	// TODO supply deletes argument from somewhere (command-line?)
	if err := fluxsync.Sync(d.Manifests, allResources, d.Cluster, false, logger); err != nil {
		logger.Log("err", err)
	}

	// Figure out which service IDs changed in this release
	changedResources := map[string]resource.Resource{}
	changedFiles, err := working.ChangedFiles(working.SyncTag)
	switch {
	case err == nil:
		// We had some changed files, we're syncing a diff
		changedResources, err = d.Manifests.LoadManifests(changedFiles...)
		if err != nil {
			logger.Log("err", errors.Wrap(err, "loading resources from repo"))
			return
		}
	case isUnknownRevision(err):
		// no synctag, We are syncing everything from scratch
		changedResources = allResources
	default:
		logger.Log("err", err)
	}
	serviceIDs := flux.ServiceIDMap{}
	for _, r := range changedResources {
		serviceIDs.Add(r.ServiceIDs(allResources))
	}

	// update notes and emit events for applied commits
	revisions, err := working.RevisionsBetween(working.SyncTag, "HEAD")
	if isUnknownRevision(err) {
		// No sync tag, grab all revisions
		revisions, err = working.RevisionsBefore("HEAD")
	}
	if err != nil {
		logger.Log("err", err)
	}

	// Emit an event
	if len(revisions) > 0 {
		if err := d.LogEvent(history.Event{
			ServiceIDs: serviceIDs.ToSlice(),
			Type:       history.EventSync,
			StartedAt:  started,
			EndedAt:    started,
			LogLevel:   history.LogLevelInfo,
			Metadata:   &history.SyncEventMetadata{Revisions: revisions},
		}); err != nil {
			logger.Log("err", err)
		}

		// Find notes in revisions.
		for i := len(revisions) - 1; i >= 0; i-- {
			n, err := working.GetNote(revisions[i])
			if err != nil {
				logger.Log("err", errors.Wrap(err, "loading notes from repo; possibly no notes"))
				// TODO: We're ignoring all errors here, not just the "no notes" error. Parse error to report proper errors.
				continue
			}
			if n == nil {
				continue
			}

			// If any of the commit notes has a release event, send
			// that to the service
			if n.Spec.Type == update.Images {
				// Map new note.Spec into ReleaseSpec
				spec := n.Spec.Spec.(update.ReleaseSpec)
				// And create a release event
				// Then wrap inside a ReleaseEventMetadata
				if err := d.LogEvent(history.Event{
					ServiceIDs: serviceIDs.ToSlice(),
					Type:       history.EventRelease,
					StartedAt:  started,
					EndedAt:    time.Now().UTC(),
					LogLevel:   history.LogLevelInfo,
					Metadata: &history.ReleaseEventMetadata{
						Revision: revisions[i],
						Spec:     spec,
						Cause:    n.Spec.Cause,
						Result:   n.Result,
						Error:    n.Result.Error(),
					},
				}); err != nil {
					logger.Log("err", err)
				}
			}
		}
	}

	// Move the tag and push it so we know how far we've gotten.
	if err := working.MoveTagAndPush("HEAD", "Sync pointer"); err != nil {
		logger.Log("err", err)
	}

	// Pull the tag if it has changed
	if err := d.updateTagRev(working, logger); err != nil {
		logger.Log("err", errors.Wrap(err, "updating tag"))
	}
}

func (d *Daemon) updateTagRev(working *git.Checkout, logger log.Logger) error {
	oldTagRev, err := d.Checkout.TagRevision(d.Checkout.SyncTag)
	if err != nil && !strings.Contains(err.Error(), "unknown revision or path not in the working tree") {
		return err
	}
	newTagRev, err := working.TagRevision(working.SyncTag)
	if err != nil {
		return err
	}

	if oldTagRev != newTagRev {
		logger.Log("tag", d.Checkout.SyncTag, "old", oldTagRev, "new", newTagRev)

		if err := d.Checkout.Pull(); err != nil {
			return err
		}
	}

	return nil
}

func isUnknownRevision(err error) bool {
	return err != nil &&
		(strings.Contains(err.Error(), "unknown revision or path not in the working tree.") ||
			strings.Contains(err.Error(), "bad revision"))
}
