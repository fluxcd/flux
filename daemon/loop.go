package daemon

import (
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"sync"

	"context"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/history"
	"github.com/weaveworks/flux/resource"
	fluxsync "github.com/weaveworks/flux/sync"
	"github.com/weaveworks/flux/update"
)

const (
	// Timeout for git operations we're prepared to abandon
	gitOpTimeout = 15 * time.Second
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

func (d *Daemon) GitPollLoop(stop chan struct{}, wg *sync.WaitGroup, logger log.Logger) {
	defer wg.Done()
	// We want to pull the repo and sync at least every
	// `GitPollInterval`. Being told to sync, or completing a job, may
	// intervene (in which case, reschedule the next pull-and-sync)
	gitPollTimer := time.NewTimer(d.GitPollInterval)
	pullThen := func(k func(logger log.Logger)) {
		defer func() {
			gitPollTimer.Stop()
			gitPollTimer = time.NewTimer(d.GitPollInterval)
		}()
		ctx, cancel := context.WithTimeout(context.Background(), gitOpTimeout)
		defer cancel()
		if err := d.Checkout.Pull(ctx); err != nil {
			logger.Log("operation", "pull", "err", err)
			return
		}
		k(logger)
	}

	imagePollTimer := time.NewTimer(d.RegistryPollInterval)

	// Ask for a sync, and to poll images, straight away
	d.askForSync()
	d.askForImagePoll()
	for {
		select {
		case <-stop:
			logger.Log("stopping", "true")
			return
		case <-d.pollImagesSoon:
			d.pollForNewImages(logger)
			imagePollTimer.Stop()
			imagePollTimer = time.NewTimer(d.RegistryPollInterval)
		case <-imagePollTimer.C:
			d.askForImagePoll()
		case <-d.syncSoon:
			pullThen(d.doSync)
		case <-gitPollTimer.C:
			// Time to poll for new commits (unless we're already
			// about to do that)
			d.askForSync()
		case job := <-d.Jobs.Ready():
			jobLogger := log.NewContext(logger).With("jobID", job.ID)
			jobLogger.Log("state", "in-progress")
			// It's assumed that (successful) jobs will push commits
			// to the upstream repo, and therefore we probably want to
			// pull from there and sync the cluster afterwards.
			if err := job.Do(jobLogger); err != nil {
				jobLogger.Log("state", "done", "success", "false", "err", err)
				continue
			}
			jobLogger.Log("state", "done", "success", "true")
			pullThen(d.doSync)
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
		working, err = d.Checkout.WorkingClone(ctx)
		if err != nil {
			logger.Log("err", err)
			return
		}
		defer working.Clean()
	}

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
		// TODO(michael): we should distinguish between "fully mostly
		// succeeded" and "failed utterly", since we want to abandon
		// this and not move the tag (and send a SyncFail event
		// upstream?), if the latter. For now, it's presumed that any
		// error returned is at worst a minor, partial failure (e.g.,
		// a small number of resources failed to sync, for unimportant
		// reasons)
	}

	// update notes and emit events for applied commits

	var initialSync bool
	var commits []git.Commit
	{
		var err error
		ctx, cancel := context.WithTimeout(ctx, gitOpTimeout)
		commits, err = working.CommitsBetween(ctx, working.SyncTag, "HEAD")
		if isUnknownRevision(err) {
			// No sync tag, grab all revisions
			initialSync = true
			commits, err = working.CommitsBefore(ctx, "HEAD")
		}
		cancel()
		if err != nil {
			logger.Log("err", err)
			return
		}
	}

	// Figure out which service IDs changed in this release
	changedResources := map[string]resource.Resource{}

	if initialSync {
		// no synctag, We are syncing everything from scratch
		changedResources = allResources
	} else {
		ctx, cancel := context.WithTimeout(ctx, gitOpTimeout)
		changedFiles, err := working.ChangedFiles(ctx, working.SyncTag)
		if err == nil {
			// We had some changed files, we're syncing a diff
			changedResources, err = d.Manifests.LoadManifests(changedFiles...)
		}
		cancel()
		if err != nil {
			logger.Log("err", errors.Wrap(err, "loading resources from repo"))
			return
		}
	}

	serviceIDs := flux.ServiceIDSet{}
	for _, r := range changedResources {
		serviceIDs.Add([]flux.ResourceID{r.ResourceID()})
	}

	var notes map[string]struct{}
	{
		ctx, cancel := context.WithTimeout(ctx, gitOpTimeout)
		notes, err = working.NoteRevList(ctx)
		cancel()
		if err != nil {
			logger.Log("err", errors.Wrap(err, "loading notes from repo"))
			return
		}
	}

	// Collect any events that come from notes attached to the commits
	// we just synced. While we're doing this, keep track of what
	// other things this sync includes e.g., releases and
	// autoreleases, that we're already posting as events, so upstream
	// can skip the sync event if it wants to.
	includes := make(map[string]bool)
	if len(commits) > 0 {
		var noteEvents []history.Event

		// Find notes in revisions.
		for i := len(commits) - 1; i >= 0; i-- {
			if _, ok := notes[commits[i].Revision]; !ok {
				includes[history.NoneOfTheAbove] = true
				continue
			}
			ctx, cancel := context.WithTimeout(ctx, gitOpTimeout)
			n, err := working.GetNote(ctx, commits[i].Revision)
			cancel()
			if err != nil {
				logger.Log("err", errors.Wrap(err, "loading notes from repo; possibly no notes"))
				return
			}
			if n == nil {
				includes[history.NoneOfTheAbove] = true
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
			case update.Images:
				spec := n.Spec.Spec.(update.ReleaseSpec)
				noteEvents = append(noteEvents, history.Event{
					ServiceIDs: serviceIDs.ToSlice(),
					Type:       history.EventRelease,
					StartedAt:  started,
					EndedAt:    time.Now().UTC(),
					LogLevel:   history.LogLevelInfo,
					Metadata: &history.ReleaseEventMetadata{
						ReleaseEventCommon: history.ReleaseEventCommon{
							Revision: commits[i].Revision,
							Result:   n.Result,
							Error:    n.Result.Error(),
						},
						Spec:  spec,
						Cause: n.Spec.Cause,
					},
				})
				includes[history.EventRelease] = true
			case update.Auto:
				spec := n.Spec.Spec.(update.Automated)
				noteEvents = append(noteEvents, history.Event{
					ServiceIDs: serviceIDs.ToSlice(),
					Type:       history.EventAutoRelease,
					StartedAt:  started,
					EndedAt:    time.Now().UTC(),
					LogLevel:   history.LogLevelInfo,
					Metadata: &history.AutoReleaseEventMetadata{
						ReleaseEventCommon: history.ReleaseEventCommon{
							Revision: commits[i].Revision,
							Result:   n.Result,
							Error:    n.Result.Error(),
						},
						Spec: spec,
					},
				})
				includes[history.EventAutoRelease] = true
			case update.Policy:
				// Use this to mean any change to policy
				includes[history.EventUpdatePolicy] = true
			default:
				// Presume it's not something we're otherwise sending
				// as an event
				includes[history.NoneOfTheAbove] = true
			}
		}

		cs := make([]history.Commit, len(commits))
		for i, c := range commits {
			cs[i].Revision = c.Revision
			cs[i].Message = c.Message
		}
		if err = d.LogEvent(history.Event{
			ServiceIDs: serviceIDs.ToSlice(),
			Type:       history.EventSync,
			StartedAt:  started,
			EndedAt:    started,
			LogLevel:   history.LogLevelInfo,
			Metadata: &history.SyncEventMetadata{
				Commits:     cs,
				InitialSync: initialSync,
				Includes:    includes,
			},
		}); err != nil {
			logger.Log("err", err)
		}

		for _, event := range noteEvents {
			if err = d.LogEvent(event); err != nil {
				logger.Log("err", err)
			}
		}
	}

	// Move the tag and push it so we know how far we've gotten.
	{
		ctx, cancel := context.WithTimeout(ctx, gitOpTimeout)
		err := working.MoveTagAndPush(ctx, "HEAD", "Sync pointer")
		cancel()
		if err != nil {
			logger.Log("err", err)
			return
		}
	}

	// Pull the tag if it has changed
	{
		ctx, cancel := context.WithTimeout(ctx, gitOpTimeout)
		if err := d.pullIfTagMoved(ctx, working, logger); err != nil {
			logger.Log("err", errors.Wrap(err, "updating tag"))
		}
		cancel()
	}
}

func (d *Daemon) pullIfTagMoved(ctx context.Context, working *git.Checkout, logger log.Logger) error {
	oldTagRev, err := d.Checkout.TagRevision(ctx, d.Checkout.SyncTag)
	if err != nil && !strings.Contains(err.Error(), "unknown revision or path not in the working tree") {
		return err
	}
	newTagRev, err := working.TagRevision(ctx, working.SyncTag)
	if err != nil {
		return err
	}

	if oldTagRev != newTagRev {
		logger.Log("tag", d.Checkout.SyncTag, "old", oldTagRev, "new", newTagRev)
		if err := d.Checkout.Pull(ctx); err != nil {
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
