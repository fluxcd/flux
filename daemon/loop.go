package daemon

import (
	"time"
)

const (
	gitPollInterval = 5 * time.Minute
)

// Loop for potentially long-running stuff. This includes running
// jobs, and looking for new commits.

func (d *Daemon) Loop(stop chan struct{}) {
	pollGit := time.Tick(gitPollInterval)
	for {
		select {
		case <-stop:
			return
		case <-pollGit:
			// Time to poll for new commits
		case job := <-d.Jobs.Ready():
			job.Do()
		}
	}
}
