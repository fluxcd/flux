package daemon

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/flux/git"
	fluxmetrics "github.com/weaveworks/flux/metrics"
	"github.com/weaveworks/flux/resource"
	fluxsync "github.com/weaveworks/flux/sync"
	"github.com/weaveworks/flux/update"
)

const (
	// Timeout for git operations we're prepared to abandon
	gitOpTimeout = 15 * time.Second
)

type LoopVars struct {
	SyncInterval         time.Duration
	RegistryPollInterval time.Duration

	initOnce       sync.Once
	syncSoon       chan struct{}
	pollImagesSoon chan struct{}
}

func (loop *LoopVars) ensureInit() {
	loop.initOnce.Do(func() {
		loop.syncSoon = make(chan struct{}, 1)
		loop.pollImagesSoon = make(chan struct{}, 1)
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
	imagePollTimer := time.NewTimer(d.RegistryPollInterval)

	// Keep track of current HEAD, so we can know when to treat a repo
	// mirror notification as a change. Otherwise, we'll just sync
	// every timer tick as well as every mirror refresh.
	syncHead := ""

	// Ask for a sync, and to poll images, straight away
	d.AskForSync()
	d.AskForImagePoll()

	for {
		var (
			lastKnownSyncTagRev      string
			warnedAboutSyncTagChange bool
		)
		select {
		case <-stop:
			logger.Log("stopping", "true")
			return
		case <-d.pollImagesSoon:
			if !imagePollTimer.Stop() {
				select {
				case <-imagePollTimer.C:
				default:
				}
			}
			d.pollForNewImages(logger)
			imagePollTimer.Reset(d.RegistryPollInterval)
		case <-imagePollTimer.C:
			d.AskForImagePoll()
		case <-d.syncSoon:
			if !syncTimer.Stop() {
				select {
				case <-syncTimer.C:
				default:
				}
			}
			if err := d.doSync(logger, &lastKnownSyncTagRev, &warnedAboutSyncTagChange); err != nil {
				logger.Log("err", err)
			}
			syncTimer.Reset(d.SyncInterval)
		case <-syncTimer.C:
			d.AskForSync()
		case <-d.Repo.C:
			ctx, cancel := context.WithTimeout(context.Background(), gitOpTimeout)
			newSyncHead, err := d.Repo.Revision(ctx, d.GitConfig.Branch)
			cancel()
			if err != nil {
				logger.Log("url", d.Repo.Origin().URL, "err", err)
				continue
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
				ctx, cancel := context.WithTimeout(context.Background(), gitOpTimeout)
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
func (d *LoopVars) AskForImagePoll() {
	d.ensureInit()
	select {
	case d.pollImagesSoon <- struct{}{}:
	default:
	}
}

// -- extra bits the loop needs

func (d *Daemon) doSync(logger log.Logger, lastKnownSyncTagRev *string, warnedAboutSyncTagChange *bool) (retErr error) {
	started := time.Now().UTC()
	defer func() {
		syncDuration.With(
			fluxmetrics.LabelSuccess, fmt.Sprint(retErr == nil),
		).Observe(time.Since(started).Seconds())
	}()
	// We don't care how long this takes overall, only about not
	// getting bogged down in certain operations, so use an
	// undeadlined context in general.
	ctx := context.Background()

	// checkout a working clone so we can mess around with tags later
	var working *git.Checkout
	{
		var err error
		ctx, cancel := context.WithTimeout(ctx, gitOpTimeout)
		defer cancel()
		working, err = d.Repo.Clone(ctx, d.GitConfig)
		if err != nil {
			return err
		}
		defer working.Clean()
	}

	// For comparison later.
	oldTagRev, err := working.SyncRevision(ctx)
	if err != nil && !isUnknownRevision(err) {
		return err
	}
	// Check if something other than the current instance of fluxd changed the sync tag.
	// This is likely to be caused by another fluxd instance using the same tag.
	// Having multiple instances fighting for the same tag can lead to fluxd missing manifest changes.
	if *lastKnownSyncTagRev != "" && oldTagRev != *lastKnownSyncTagRev && !*warnedAboutSyncTagChange {
		logger.Log("warning",
			"detected external change in git sync tag; the sync tag should not be shared by fluxd instances")
		*warnedAboutSyncTagChange = true
	}

	newTagRev, err := working.HeadRevision(ctx)
	if err != nil {
		return err
	}

	// Get a map of all resources defined in the repo
	allResources, err := d.Manifests.LoadManifests(working.Dir(), working.ManifestDirs())
	if err != nil {
		return errors.Wrap(err, "loading resources from repo")
	}

	var resourceErrors []event.ResourceError
	// TODO supply deletes argument from somewhere (command-line?)
	if err := fluxsync.Sync(logger, d.Manifests, allResources, d.Cluster, false); err != nil {
		logger.Log("err", err)
		switch syncerr := err.(type) {
		case cluster.SyncError:
			for _, e := range syncerr {
				resourceErrors = append(resourceErrors, event.ResourceError{
					ID:    e.ResourceID(),
					Path:  e.Source(),
					Error: e.Error.Error(),
				})
			}
		default:
			return err
		}
	}

	// update notes and emit events for applied commits

	var initialSync bool
	var commits []git.Commit
	{
		var err error
		ctx, cancel := context.WithTimeout(ctx, gitOpTimeout)
		if oldTagRev != "" {
			commits, err = d.Repo.CommitsBetween(ctx, oldTagRev, newTagRev, d.GitConfig.Paths...)
		} else {
			initialSync = true
			commits, err = d.Repo.CommitsBefore(ctx, newTagRev, d.GitConfig.Paths...)
		}
		cancel()
		if err != nil {
			return err
		}
	}

	// Figure out which service IDs changed in this release
	changedResources := map[string]resource.Resource{}

	if initialSync {
		// no synctag, We are syncing everything from scratch
		changedResources = allResources
	} else {
		ctx, cancel := context.WithTimeout(ctx, gitOpTimeout)
		changedFiles, err := working.ChangedFiles(ctx, oldTagRev)
		if err == nil && len(changedFiles) > 0 {
			// We had some changed files, we're syncing a diff
			// FIXME(michael): this won't be accurate when a file can have more than one resource
			changedResources, err = d.Manifests.LoadManifests(working.Dir(), changedFiles)
		}
		cancel()
		if err != nil {
			return errors.Wrap(err, "loading resources from repo")
		}
	}

	serviceIDs := flux.ResourceIDSet{}
	for _, r := range changedResources {
		serviceIDs.Add([]flux.ResourceID{r.ResourceID()})
	}

	var notes map[string]struct{}
	{
		ctx, cancel := context.WithTimeout(ctx, gitOpTimeout)
		notes, err = working.NoteRevList(ctx)
		cancel()
		if err != nil {
			return errors.Wrap(err, "loading notes from repo")
		}
	}

	// Collect any events that come from notes attached to the commits
	// we just synced. While we're doing this, keep track of what
	// other things this sync includes e.g., releases and
	// autoreleases, that we're already posting as events, so upstream
	// can skip the sync event if it wants to.
	includes := make(map[string]bool)
	if len(commits) > 0 {
		var noteEvents []event.Event

		// Find notes in revisions.
		for i := len(commits) - 1; i >= 0; i-- {
			if _, ok := notes[commits[i].Revision]; !ok {
				includes[event.NoneOfTheAbove] = true
				continue
			}
			ctx, cancel := context.WithTimeout(ctx, gitOpTimeout)
			var n note
			ok, err := working.GetNote(ctx, commits[i].Revision, &n)
			cancel()
			if err != nil {
				return errors.Wrap(err, "loading notes from repo")
			}
			if !ok {
				includes[event.NoneOfTheAbove] = true
				continue
			}

			// If this is the first sync, we should expect no notes,
			// since this is supposedly the first time we're seeing
			// the repo. But there are circumstances in which we can
			// nonetheless see notes -- if the tag was deleted from
			// the upstream repo, or if this accidentally has the same
			// notes ref as another daemon using the same repo (but a
			// different tag). Either way, we don't want to report any
			// notes on an initial sync, since they (most likely)
			// don't belong to us.
			if initialSync {
				logger.Log("warning", "no notes expected on initial sync; this repo may be in use by another fluxd")
				break
			}

			// Interpret some notes as events to send to the upstream
			switch n.Spec.Type {
			case update.Containers:
				spec := n.Spec.Spec.(update.ReleaseContainersSpec)
				noteEvents = append(noteEvents, event.Event{
					ServiceIDs: n.Result.AffectedResources(),
					Type:       event.EventRelease,
					StartedAt:  started,
					EndedAt:    time.Now().UTC(),
					LogLevel:   event.LogLevelInfo,
					Metadata: &event.ReleaseEventMetadata{
						ReleaseEventCommon: event.ReleaseEventCommon{
							Revision: commits[i].Revision,
							Result:   n.Result,
							Error:    n.Result.Error(),
						},
						Spec: event.ReleaseSpec{
							Type:                  event.ReleaseContainersSpecType,
							ReleaseContainersSpec: &spec,
						},
						Cause: n.Spec.Cause,
					},
				})
				includes[event.EventRelease] = true
			case update.Images:
				spec := n.Spec.Spec.(update.ReleaseImageSpec)
				noteEvents = append(noteEvents, event.Event{
					ServiceIDs: n.Result.AffectedResources(),
					Type:       event.EventRelease,
					StartedAt:  started,
					EndedAt:    time.Now().UTC(),
					LogLevel:   event.LogLevelInfo,
					Metadata: &event.ReleaseEventMetadata{
						ReleaseEventCommon: event.ReleaseEventCommon{
							Revision: commits[i].Revision,
							Result:   n.Result,
							Error:    n.Result.Error(),
						},
						Spec: event.ReleaseSpec{
							Type:             event.ReleaseImageSpecType,
							ReleaseImageSpec: &spec,
						},
						Cause: n.Spec.Cause,
					},
				})
				includes[event.EventRelease] = true
			case update.Auto:
				spec := n.Spec.Spec.(update.Automated)
				noteEvents = append(noteEvents, event.Event{
					ServiceIDs: n.Result.AffectedResources(),
					Type:       event.EventAutoRelease,
					StartedAt:  started,
					EndedAt:    time.Now().UTC(),
					LogLevel:   event.LogLevelInfo,
					Metadata: &event.AutoReleaseEventMetadata{
						ReleaseEventCommon: event.ReleaseEventCommon{
							Revision: commits[i].Revision,
							Result:   n.Result,
							Error:    n.Result.Error(),
						},
						Spec: spec,
					},
				})
				includes[event.EventAutoRelease] = true
			case update.Policy:
				// Use this to mean any change to policy
				includes[event.EventUpdatePolicy] = true
			default:
				// Presume it's not something we're otherwise sending
				// as an event
				includes[event.NoneOfTheAbove] = true
			}
		}

		cs := make([]event.Commit, len(commits))
		for i, c := range commits {
			cs[i].Revision = c.Revision
			cs[i].Message = c.Message
		}
		if err = d.LogEvent(event.Event{
			ServiceIDs: serviceIDs.ToSlice(),
			Type:       event.EventSync,
			StartedAt:  started,
			EndedAt:    started,
			LogLevel:   event.LogLevelInfo,
			Metadata: &event.SyncEventMetadata{
				Commits:     cs,
				InitialSync: initialSync,
				Includes:    includes,
				Errors:      resourceErrors,
			},
		}); err != nil {
			logger.Log("err", err)
			// Abort early to ensure at least once delivery of events
			return err
		}

		for _, event := range noteEvents {
			if err = d.LogEvent(event); err != nil {
				logger.Log("err", err)
				// Abort early to ensure at least once delivery of events
				return err
			}
		}
	}

	// Move the tag and push it so we know how far we've gotten.
	if oldTagRev != newTagRev {
		{
			ctx, cancel := context.WithTimeout(ctx, gitOpTimeout)
			err := working.MoveSyncTagAndPush(ctx, newTagRev, "Sync pointer")
			cancel()
			if err != nil {
				return err
			}
			*lastKnownSyncTagRev = newTagRev
		}
		logger.Log("tag", d.GitConfig.SyncTag, "old", oldTagRev, "new", newTagRev)
		{
			ctx, cancel := context.WithTimeout(ctx, gitOpTimeout)
			err := d.Repo.Refresh(ctx)
			cancel()
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
