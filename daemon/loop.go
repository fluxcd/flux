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
	for {
		select {
		case <-stop:
			logger.Log("stopping", "true")
			return
		case <-pollGit.C:
			// Time to poll for new commits
			d.PullAndSync(logger)
			resetGitPoll()
		case <-pollImages:
			// Time to poll for new images
			d.PollImages()
		case job := <-d.Jobs.Ready():
			// It's assumed that (successful) jobs will push commits
			// to the upstream repo, and therefore we probably want to
			// pull from there and sync the cluster.
			if err := job.Do(); err != nil {
				logger.Log("err", err)
				continue
			}
			d.PullAndSync(logger)
			resetGitPoll()
		}
	}
}

func (d *Daemon) PullAndSync(logger log.Logger) {
	if err := d.Checkout.Pull(); err != nil {
		logger.Log("err", err)
	}
	// TODO supply deletes argument from somewhere (command-line?)
	if err := sync.Sync(d.Checkout.ManifestDir(), d.Cluster, false); err != nil {
		logger.Log("err", err)
	}
}
