package daemon

import (
	"context"
	"fmt"
	"github.com/weaveworks/flux/git"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	fluxmetrics "github.com/weaveworks/flux/metrics"
)

type LoopVars struct {
	SyncInterval        time.Duration
	AutomationInterval  time.Duration
	GitTimeout          time.Duration
	GitVerifySignatures bool

	initOnce               sync.Once
	syncSoon               chan struct{}
	automatedWorkloadsSoon chan struct{}
}

func (loop *LoopVars) ensureInit() {
	loop.initOnce.Do(func() {
		loop.syncSoon = make(chan struct{}, 1)
		loop.automatedWorkloadsSoon = make(chan struct{}, 1)
	})
}

func (d *Daemon) Loop(stop chan struct{}, wg *sync.WaitGroup, logger log.Logger) {
	defer wg.Done()

	// We want to sync at least every `SyncInterval`. Being told to
	// sync, or completing a job, may intervene (in which case,
	// reschedule the next sync).
	syncTimer := time.NewTimer(d.SyncInterval)
	// Similarly checking to see if any controllers have new images
	// available.
	automatedWorkloadTimer := time.NewTimer(d.AutomationInterval)

	// Keep track of current, verified (if signature verification is
	// enabled), HEAD, so we can know when to treat a repo
	// mirror notification as a change. Otherwise, we'll just sync
	// every timer tick as well as every mirror refresh.
	syncHead := ""

	// In-memory sync tag state
	lastKnownSyncTag := &lastKnownSyncTag{logger: logger, syncTag: d.GitConfig.SyncTag}

	// Ask for a sync, and to check
	d.AskForSync()
	d.AskForAutomatedWorkloadImageUpdates()

	for {
		select {
		case <-stop:
			logger.Log("stopping", "true")
			return
		case <-d.automatedWorkloadsSoon:
			if !automatedWorkloadTimer.Stop() {
				select {
				case <-automatedWorkloadTimer.C:
				default:
				}
			}
			d.pollForNewAutomatedWorkloadImages(logger)
			automatedWorkloadTimer.Reset(d.AutomationInterval)
		case <-automatedWorkloadTimer.C:
			d.AskForAutomatedWorkloadImageUpdates()
		case <-d.syncSoon:
			if !syncTimer.Stop() {
				select {
				case <-syncTimer.C:
				default:
				}
			}
			started := time.Now().UTC()
			err := d.Sync(context.Background(), started, syncHead, lastKnownSyncTag)
			syncDuration.With(
				fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
			).Observe(time.Since(started).Seconds())
			if err != nil {
				logger.Log("err", err)
			}
			syncTimer.Reset(d.SyncInterval)
		case <-syncTimer.C:
			d.AskForSync()
		case <-d.Repo.C:
			var newSyncHead string
			var invalidCommit git.Commit
			var err error

			ctx, cancel := context.WithTimeout(context.Background(), d.GitTimeout)
			if d.GitVerifySignatures {
				newSyncHead, invalidCommit, err = latestValidRevision(ctx, d.Repo, d.GitConfig)
			} else {
				newSyncHead, err = d.Repo.BranchHead(ctx)
			}
			cancel()

			if err != nil {
				logger.Log("url", d.Repo.Origin().URL, "err", err)
				continue
			}
			if invalidCommit.Revision != "" {
				logger.Log("err", "found invalid GPG signature for commit", "revision", invalidCommit.Revision, "key", invalidCommit.Signature.Key)
			}

			logger.Log("event", "refreshed", "url", d.Repo.Origin().URL, "branch", d.GitConfig.Branch, "HEAD", newSyncHead)
			if newSyncHead != syncHead {
				syncHead = newSyncHead
				d.AskForSync()
			}
		case job := <-d.Jobs.Ready():
			queueLength.Set(float64(d.Jobs.Len()))
			jobLogger := log.With(logger, "jobID", job.ID)
			jobLogger.Log("state", "in-progress")
			// It's assumed that (successful) jobs will push commits
			// to the upstream repo, and therefore we probably want to
			// pull from there and sync the cluster afterwards.
			start := time.Now()
			err := job.Do(jobLogger)
			jobDuration.With(
				fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
			).Observe(time.Since(start).Seconds())
			if err != nil {
				jobLogger.Log("state", "done", "success", "false", "err", err)
			} else {
				jobLogger.Log("state", "done", "success", "true")
				ctx, cancel := context.WithTimeout(context.Background(), d.GitTimeout)
				err := d.Repo.Refresh(ctx)
				if err != nil {
					logger.Log("err", err)
				}
				cancel()
			}
		}
	}
}

// Ask for a sync, or if there's one waiting, let that happen.
func (d *LoopVars) AskForSync() {
	d.ensureInit()
	select {
	case d.syncSoon <- struct{}{}:
	default:
	}
}

// Ask for an image poll, or if there's one waiting, let that happen.
func (d *LoopVars) AskForAutomatedWorkloadImageUpdates() {
	d.ensureInit()
	select {
	case d.automatedWorkloadsSoon <- struct{}{}:
	default:
	}
}

// -- internals to keep track of sync tag state
type lastKnownSyncTag struct {
	logger            log.Logger
	syncTag           string
	revision          string
	warnedAboutChange bool
}

// SetRevision updates the sync tag revision in git _and_ the
// in-memory revision, if it has changed. In addition, it validates
// if the in-memory revision matches the old revision from git before
// making the update, to notify a user about multiple Flux daemons
// using the same tag.
func (s *lastKnownSyncTag) SetRevision(ctx context.Context, working *git.Checkout, timeout time.Duration,
	oldRev, newRev string) (bool, error) {
	// Check if something other than the current instance of fluxd
	// changed the sync tag. This is likely caused by another instance
	// using the same tag. Having multiple instances fight for the same
	// tag can lead to fluxd missing manifest changes.
	if s.revision != "" && oldRev != s.revision && !s.warnedAboutChange {
		s.logger.Log("warning",
			"detected external change in git sync tag; the sync tag should not be shared by fluxd instances",
			"tag", s.syncTag)
		s.warnedAboutChange = true
	}

	// Did it actually change?
	if s.revision == newRev {
		return false, nil
	}

	// Update the sync tag revision in git
	tagAction := git.TagAction{
		Revision: newRev,
		Message:  "Sync pointer",
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	if err := working.MoveSyncTagAndPush(ctx, tagAction); err != nil {
		return false, err
	}
	cancel()

	// Update in-memory revision
	s.revision = newRev

	s.logger.Log("tag", s.syncTag, "old", oldRev, "new", newRev)
	return true, nil
}
