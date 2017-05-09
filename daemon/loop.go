package daemon

import (
	"strings"
	"time"

	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux/history"
	"github.com/weaveworks/flux/sync"
)

const (
	gitPollInterval    = 5 * time.Minute
	imagesPollInterval = 5 * time.Minute
)

// Loop for potentially long-running stuff. This includes running
// jobs, and looking for new commits.

func (d *Daemon) Loop(stop chan struct{}, logger log.Logger) {
	pollGit := time.NewTimer(gitPollInterval)
	resetGitPoll := func() {
		if pollGit != nil {
			pollGit.Stop()
			pollGit = time.NewTimer(gitPollInterval)
		}
	}

	pollImages := time.Tick(imagesPollInterval)
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
			d.PollImages()
		case job := <-d.Jobs.Ready():
			logger.Log("job", job.ID)
			// It's assumed that (successful) jobs will push commits
			// to the upstream repo, and therefore we probably want to
			// pull from there and sync the cluster.
			if err := job.Do(); err != nil {
				logger.Log("job", job.ID, "err", err)
				continue
			}
			logger.Log("job", job.ID, "success", "true")
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

	// update notes and emit events for applied commits
	revisions, err := working.RevisionsBetween(working.SyncTag+"~1", "HEAD")
	if isUnknownRevision(err) {
		// No sync tag, grab all revisions
		revisions, err = working.RevisionsBefore("HEAD")
	}
	if err != nil {
		logger.Log("err", err)
	}

	// TODO supply deletes argument from somewhere (command-line?)
	changedFiles, err := working.ChangedFiles(working.SyncTag)
	if isUnknownRevision(err) {
		// no synctag, list all files
		changedFiles = []string{working.ManifestDir()}
	} else if err != nil {
		logger.Log("err", err)
	}
	if len(changedFiles) == 0 {
		return
	}
	serviceIDs, err := sync.Sync(changedFiles, d.Cluster, false)
	if err != nil {
		logger.Log("err", err)
	}

	// Emit an event
	if len(revisions) > 0 {
		if err := d.LogEvent(history.Event{
			ServiceIDs: serviceIDs,
			Type:       history.EventSync,
			StartedAt:  started,
			EndedAt:    started,
			LogLevel:   history.LogLevelInfo,
			Metadata:   &history.SyncEventMetadata{Revisions: revisions},
		}); err != nil {
			logger.Log("err", err)
		}
	}

	// Move the tag and push it so we know how far we've gotten.
	if err := working.MoveTagAndPush("HEAD", "Sync pointer"); err != nil {
		logger.Log("err", err)
	}
}

func isUnknownRevision(err error) bool {
	return err != nil &&
		(strings.Contains(err.Error(), "unknown revision or path not in the working tree.") ||
			strings.Contains(err.Error(), "bad revision"))
}
