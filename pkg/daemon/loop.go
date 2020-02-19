package daemon

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/kit/log"

	"github.com/fluxcd/flux/pkg/git"
	fluxmetrics "github.com/fluxcd/flux/pkg/metrics"
	fluxsync "github.com/fluxcd/flux/pkg/sync"
)

type LoopVars struct {
	SyncInterval            time.Duration
	SyncTimeout             time.Duration
	AutomationInterval      time.Duration
	GitTimeout              time.Duration
	GitVerifySignaturesMode fluxsync.VerifySignaturesMode
	SyncState               fluxsync.State
	ImageScanDisabled       bool

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
	ratchet := &lastKnownSyncState{logger: logger, state: d.SyncState}

	// If the git repo is read-only, the image updates will fail; to
	// avoid repeated failures in the log, mention it here and
	// otherwise skip it when it comes around.
	if d.Repo.Readonly() {
		logger.Log("info", "Repo is read-only; no image updates will be attempted")
	}

	// Same for registry scanning
	if d.ImageScanDisabled {
		logger.Log("info", "Registry scanning is disabled; no image updates will be attempted")
	}

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
			if d.Repo.Readonly() || d.ImageScanDisabled {
				// don't bother trying to update images, and don't
				// bother setting the timer again
				continue
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
			err := d.Sync(context.Background(), started, syncHead, ratchet)
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
			if d.GitVerifySignaturesMode != fluxsync.VerifySignaturesModeNone {
				newSyncHead, invalidCommit, err = latestValidRevision(ctx, d.Repo, d.SyncState, d.GitVerifySignaturesMode)
			} else {
				newSyncHead, err = d.Repo.BranchHead(ctx)
			}
			cancel()

			if err != nil {
				logger.Log("url", d.Repo.Origin().SafeURL(), "err", err)
				continue
			}
			if invalidCommit.Revision != "" {
				logger.Log("err", "found invalid GPG signature for commit", "revision", invalidCommit.Revision, "key", invalidCommit.Signature.Key)
			}

			logger.Log("event", "refreshed", "url", d.Repo.Origin().SafeURL(), "branch", d.GitConfig.Branch, "HEAD", newSyncHead)
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
type lastKnownSyncState struct {
	logger log.Logger
	state  fluxsync.State

	// bookkeeping
	revision          string
	warnedAboutChange bool
}

// Current returns the revision from the state
func (s *lastKnownSyncState) Current(ctx context.Context) (string, error) {
	return s.state.GetRevision(ctx)
}

// Update records the synced revision in persistent storage (the
// sync.State). In addition, it checks that the old revision matches
// the last sync revision before making the update; mismatches suggest
// multiple Flux daemons are using the same state, so we log these.
func (s *lastKnownSyncState) Update(ctx context.Context, oldRev, newRev string) (bool, error) {
	// Check if something other than the current instance of fluxd
	// changed the sync tag. This is likely caused by another instance
	// using the same tag. Having multiple instances fight for the same
	// tag can lead to fluxd missing manifest changes.
	if s.revision != "" && oldRev != s.revision && !s.warnedAboutChange {
		s.logger.Log("warning",
			"detected external change in sync state; the sync state should not be shared by fluxd instances",
			"state", s.state.String())
		s.warnedAboutChange = true
	}

	// Did it actually change?
	if s.revision == newRev {
		return false, nil
	}

	if err := s.state.UpdateMarker(ctx, newRev); err != nil {
		return false, err
	}

	// Update in-memory revision
	s.revision = newRev

	s.logger.Log("state", s.state.String(), "old", oldRev, "new", newRev)
	return true, nil
}
