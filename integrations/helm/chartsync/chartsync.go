/*
Package sync provides the functionality for updating a Chart release
due to (git repo) changes of Charts, while no Custom Resource changes.

Helm operator regularly checks the Chart repo and if new commits are found
all Custom Resources related to the changed Charts are updates, resulting in new
Chart release(s).
*/
package chartsync

import (
	"log"
	"sync"
	"time"

	chartrelease "github.com/weaveworks/flux/integrations/helm/release"
)

type ChartChangeSync struct {
	logger       log.Logger
	PollInterval time.Duration
	PollTimeout  time.Duration
	release      chartrelease.Release
}

//  Run ... create a syncing loop
func (chs *ChartChangeSync) Run(stopCh <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()

	// every UpdateInterval check the git repo

	// 		get all FHR resources
	//				collect all gitchartpaths into a list ($Dir/$chartPath)

	//

	/*
		pollTimer := time.NewTimer(chs.PollInterval)
		pullThen := func(k func(logger log.Logger) error) {
			defer func() {
				pollTimer.Stop()
				pollTimer = time.NewTimer(chs.PollInterval)
			}()
			ctx, cancel := context.WithTimeout(context.Background(), gitOpTimeout)
			defer cancel()
			if err := chs.release.Pull(ctx); err != nil {
				logger.Log("operation", "pull", "err", err)
				return
			}
			if err := k(logger); err != nil {
				logger.Log("operation", "after-pull", "err", err)
			}
		}
	*/
}
