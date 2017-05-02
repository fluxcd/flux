package daemon

import (
	"time"

	"github.com/go-kit/kit/log"

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
	if err := d.Checkout.Pull(); err != nil {
		logger.Log("err", err)
	}
	// TODO supply deletes argument from somewhere (command-line?)
	if err := sync.Sync(d.Checkout.ManifestDir(), d.Cluster, false); err != nil {
		logger.Log("err", err)
	}
}
