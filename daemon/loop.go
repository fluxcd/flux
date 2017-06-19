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

type LoopVars struct {
	GitPollInterval      time.Duration
	RegistryPollInterval time.Duration
	syncSoon             chan struct{}
	pollImagesSoon       chan struct{}
	initOnce             sync.Once
}

func (loop *LoopVars) ensureInit() {
	loop.initOnce.Do(func() {
		loop.syncSoon = make(chan struct{}, 1)
		loop.pollImagesSoon = make(chan struct{}, 1)
	})
}

// Loop for potentially long-running stuff. This includes running
// jobs, and looking for new commits.

func (d *Daemon) Loop(stop chan struct{}, wg *sync.WaitGroup, logger log.Logger) {
	defer wg.Done()
	gitPollTimer := time.NewTimer(d.GitPollInterval)
	// This can be called any time to push the next git poll out by
	// d.GitPollInterval. It must _at least_ be called when the timer
	// fires, so that the polling continues.
	resetGitPoll := func() {
		if gitPollTimer != nil {
			gitPollTimer.Stop()
			gitPollTimer = time.NewTimer(d.GitPollInterval)
		}
	}

	// Pull for new commits, then if successful, run the procedure given
	pullThen := func(k func(logger log.Logger)) {
		if err := d.Checkout.Pull(); err != nil {
			logger.Log("operation", "pull", "err", err)
			return
		}
		k(logger)
	}

	imagePollTimer := time.NewTimer(d.RegistryPollInterval)
	// Similar to resetGitTimer, this must at lest be called when the
	// timer fires.
	resetImagePoll := func() {
		if imagePollTimer != nil {
			imagePollTimer.Stop()
			imagePollTimer = time.NewTimer(d.RegistryPollInterval)
		}
	}

	// Ask for a sync, and to poll images, straight away
	d.askForSync()
	d.askForImagePoll()
	for {
		select {
		case <-stop:
			logger.Log("stopping", "true")
			return
		case <-d.syncSoon:
			pullThen(d.doSync)
			resetGitPoll()
		case <-d.pollImagesSoon:
			pullThen(d.pollForNewImages)
			resetGitPoll()
		case <-gitPollTimer.C:
			// Time to poll for new commits (unless we're already
			// about to do that)
			d.askForSync()
		case <-imagePollTimer.C:
			// Time to poll for new images
			d.askForImagePoll()
			resetImagePoll()
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
			pullThen(d.doSync)
			resetGitPoll()
		}
	}
}

// Ask for a sync, or if there's one waiting, let that happen.
func (d *LoopVars) askForSync() {
	d.ensureInit()
	select {
	case d.syncSoon <- struct{}{}:
	default:
	}
}

// Ask for an image poll, or if there's one waiting, let that happen.
func (d *LoopVars) askForImagePoll() {
	d.ensureInit()
	select {
	case d.pollImagesSoon <- struct{}{}:
	default:
	}
}

// -- extra bits the loop needs

func (d *Daemon) doSync(logger log.Logger) {
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
	serviceIDs := flux.ServiceIDSet{}
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
			switch n.Spec.Type {
			case update.Images:
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
						ReleaseEventCommon: history.ReleaseEventCommon{
							Revision: revisions[i],
							Result:   n.Result,
							Error:    n.Result.Error(),
						},
						Spec:  spec,
						Cause: n.Spec.Cause,
					},
				}); err != nil {
					logger.Log("err", err)
				}
			case update.Auto:
				spec := n.Spec.Spec.(update.Automated)
				if err := d.LogEvent(history.Event{
					ServiceIDs: serviceIDs.ToSlice(),
					Type:       history.EventAutoRelease,
					StartedAt:  started,
					EndedAt:    time.Now().UTC(),
					LogLevel:   history.LogLevelInfo,
					Metadata: &history.AutoReleaseEventMetadata{
						ReleaseEventCommon: history.ReleaseEventCommon{
							Revision: revisions[i],
							Result:   n.Result,
							Error:    n.Result.Error(),
						},
						Spec: spec,
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
