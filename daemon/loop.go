package daemon

import (
	"time"
)

const (
	gitPollInterval    = 5 * time.Minute
	imagesPollInterval = 5 * time.Minute
)

// Loop for potentially long-running stuff. This includes running
// jobs, and looking for new commits.

func (d *Daemon) Loop(stop chan struct{}) {
	pollGit := time.Tick(gitPollInterval)
	pollImages := time.Tick(imagesPollInterval)
	for {
		select {
		case <-stop:
			return
		case <-pollGit:
			// Time to poll for new commits
		case <-pollImages:
			// Time to poll for new images
			d.PollImages()
		case job := <-d.Jobs.Ready():
			job.Do()
		}
	}
}
