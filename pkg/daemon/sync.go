package daemon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/fluxcd/flux/pkg/metrics"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/fluxcd/flux/pkg/cluster"
	"github.com/fluxcd/flux/pkg/event"
	"github.com/fluxcd/flux/pkg/git"
	"github.com/fluxcd/flux/pkg/manifests"
	"github.com/fluxcd/flux/pkg/resource"
	fluxsync "github.com/fluxcd/flux/pkg/sync"
	"github.com/fluxcd/flux/pkg/update"
)

// ratchet is for keeping track of transitions between
// revisions. This is slightly more complicated than just setting the
// state, since we want to notice unexpected transitions (e.g., when
// the apparent current state is not what we'd recorded).
type ratchet interface {
	CurrentRevision(ctx context.Context) (string, error)
	CurrentResources() map[string]resource.Resource

	Update(ctx context.Context, oldRev, newRev string, resources map[string]resource.Resource) (bool, error)
}

type eventLogger interface {
	LogEvent(e event.Event) error
}

type changeSet struct {
	commits     []git.Commit
	oldTagRev   string
	newTagRev   string
	initialSync bool
}

// Sync starts the synchronization of the cluster with git.
func (d *Daemon) Sync(ctx context.Context, started time.Time, newRevision string, rat ratchet) error {
	// Load last-synced resources for comparison
	lastResources, err := d.getLastResources(ctx, rat)
	if err != nil {
		d.Logger.Log("warning", "failed to load last-synced resources. sync event may be inaccurate", "err", err)
		lastResources = map[string]resource.Resource{}
	}

	// Retrieve change set of commits we need to sync
	changeSet, err := d.getChangeSet(ctx, rat, newRevision)
	if err != nil {
		return err
	}

	d.Logger.Log("info", "trying to sync git changes to the cluster", "old", changeSet.oldTagRev, "new", changeSet.newTagRev)

	// Load resources from the new revision
	resourceStore, cleanup, err := d.getManifestStoreByRevision(ctx, newRevision)
	if err != nil {
		return errors.Wrap(err, "loading new resources")
	}
	defer cleanup()

	// Run actual sync of resources on cluster
	syncSetName := makeGitConfigHash(d.Repo.Origin(), d.GitConfig)
	resources, resourceErrors, err := doSync(ctx, resourceStore, d.Cluster, syncSetName, d.Logger)
	if err != nil {
		return err
	}

	// Determine what resources changed and deleted during the sync
	updatedIDs, deletedIDs := compareResources(lastResources, resources)
	// TODO(ordovicia): include deleted resources in sync events
	_ = deletedIDs

	// Retrieve git notes and collect events from them
	notes, err := d.getNotes(ctx, d.GitTimeout)
	if err != nil {
		return err
	}
	noteEvents, includesEvents, err := d.collectNoteEvents(ctx, changeSet, notes, d.GitTimeout, started, d.Logger)
	if err != nil {
		return err
	}

	// Report all synced commits
	if err := logCommitEvent(d, changeSet, updatedIDs, started, includesEvents, resourceErrors, d.Logger); err != nil {
		return err
	}

	// Report all collected events
	for _, event := range noteEvents {
		if err = d.LogEvent(event); err != nil {
			d.Logger.Log("err", err)
			// Abort early to ensure at least once delivery of events
			return err
		}
	}

	// Move the revision the sync state points to
	if ok, err := rat.Update(ctx, changeSet.oldTagRev, changeSet.newTagRev, resources); err != nil {
		return err
	} else if !ok {
		return nil
	}

	err = refresh(ctx, d.GitTimeout, d.Repo)
	return err
}

// getLastResources loads last-synced resources
func (d *Daemon) getLastResources(ctx context.Context, rat ratchet) (map[string]resource.Resource, error) {
	lastResources := rat.CurrentResources()
	if lastResources != nil {
		return lastResources, nil
	}

	currentRevision, err := rat.CurrentRevision(ctx)
	if err != nil {
		return nil, err
	}

	// Repo has never been cloned yet
	if currentRevision == "" {
		return make(map[string]resource.Resource), nil
	}

	// Fist sync -- load resources from clone of currentRevision
	lastResourcestore, cleanup, err := d.getManifestStoreByRevision(ctx, currentRevision)
	if err != nil {
		return nil, errors.Wrap(err, "reading the repository checkout")
	}
	defer cleanup()

	lastResources, err = lastResourcestore.GetAllResourcesByID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "loading resources from repo")
	}

	return lastResources, nil
}

// getManifestStoreByRevision loads manifests from a clone of a given revision.
func (d *Daemon) getManifestStoreByRevision(ctx context.Context, revision string) (store manifests.Store, cleanupClone func(), err error) {
	clone, cleanupClone, err := d.cloneRepo(ctx, revision)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cloning repo")
	}

	store, err = d.getManifestStore(clone)
	return store, cleanupClone, err
}

// cloneRepo makes a read-only clone of the given revision
func (d *Daemon) cloneRepo(ctx context.Context, revision string) (clone *git.Export, cleanup func(), err error) {
	ctxGitOp, cancel := context.WithTimeout(ctx, d.GitTimeout)
	defer cancel()
	clone, err = d.Repo.Export(ctxGitOp, revision)
	if err != nil {
		return nil, nil, err
	}

	// Unseal any secrets if enabled
	if d.GitSecretEnabled {
		ctxGitOp, cancel := context.WithTimeout(ctx, d.GitTimeout)
		defer cancel()
		if err := clone.SecretUnseal(ctxGitOp); err != nil {
			return nil, nil, err
		}
	}

	cleanup = func() {
		if err := clone.Clean(); err != nil {
			d.Logger.Log("error", fmt.Sprintf("cannot clean clone: %s", err))
		}
	}

	return clone, cleanup, nil
}

// getChangeSet returns the change set of commits for this sync,
// including the revision range and if it is an initial sync.
func (d *Daemon) getChangeSet(ctxGitOp context.Context, state ratchet, headRev string) (changeSet, error) {
	var c changeSet
	var err error

	currentRev, err := state.CurrentRevision(ctxGitOp)
	if err != nil {
		return c, err
	}

	c.oldTagRev = currentRev
	c.newTagRev = headRev

	paths := d.GitConfig.Paths
	if d.ManifestGenerationEnabled {
		paths = []string{}
	}

	ctxGitOp, cancel := context.WithTimeout(ctxGitOp, d.GitTimeout)
	if c.oldTagRev != "" {
		c.commits, err = d.Repo.CommitsBetween(ctxGitOp, c.oldTagRev, c.newTagRev, false, paths...)
	} else {
		c.initialSync = true
		c.commits, err = d.Repo.CommitsBefore(ctxGitOp, c.newTagRev, false, paths...)
	}
	cancel()

	return c, err
}

// doSync runs the actual sync of workloads on the cluster. It returns
// a map with all resources it applied and sync errors it encountered.
func doSync(ctx context.Context, manifestsStore manifests.Store, clus cluster.Cluster, syncSetName string,
	logger log.Logger) (map[string]resource.Resource, []event.ResourceError, error) {
	resources, err := manifestsStore.GetAllResourcesByID(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "loading resources from repo")
	}

	var resourceErrors []event.ResourceError
	if err := fluxsync.Sync(syncSetName, resources, clus); err != nil {
		switch syncerr := err.(type) {
		case cluster.SyncError:
			logger.Log("err", err)
			updateSyncManifestsMetric(len(resources)-len(syncerr), len(syncerr))
			for _, e := range syncerr {
				resourceErrors = append(resourceErrors, event.ResourceError{
					ID:    e.ResourceID,
					Path:  e.Source,
					Error: e.Error.Error(),
				})
			}
		default:
			return nil, nil, err
		}
	} else {
		updateSyncManifestsMetric(len(resources), 0)
	}
	return resources, resourceErrors, nil
}

func updateSyncManifestsMetric(success, failure int) {
	syncManifestsMetric.With(metrics.LabelSuccess, "true").Set(float64(success))
	syncManifestsMetric.With(metrics.LabelSuccess, "false").Set(float64(failure))
}

func compareResources(old, new map[string]resource.Resource) (updated, deleted resource.IDSet) {
	updated, deleted = resource.IDSet{}, resource.IDSet{}
	toIDs := func(r resource.Resource) []resource.ID { return []resource.ID{r.ResourceID()} }

	for newID, newResource := range new {
		if oldResource, ok := old[newID]; ok {
			if !bytes.Equal(oldResource.Bytes(), newResource.Bytes()) {
				updated.Add(toIDs(newResource))
			}
		} else {
			updated.Add(toIDs(newResource))
		}
	}

	for oldID, oldResource := range old {
		if _, ok := new[oldID]; !ok {
			deleted.Add(toIDs(oldResource))
		}
	}

	return updated, deleted
}

// getNotes retrieves the git notes from the working clone.
func (d *Daemon) getNotes(ctx context.Context, timeout time.Duration) (map[string]struct{}, error) {
	ctxGitOp, cancel := context.WithTimeout(ctx, timeout)
	notes, err := d.Repo.NoteRevList(ctxGitOp, d.GitConfig.NotesRef)
	cancel()
	if err != nil {
		return nil, errors.Wrap(err, "loading notes from repo")
	}
	return notes, nil
}

// collectNoteEvents collects any events that come from notes attached
// to the commits we just synced. While we're doing this, keep track
// of what other things this sync includes e.g., releases and
// autoreleases, that we're already posting as events, so upstream
// can skip the sync event if it wants to.
func (d *Daemon) collectNoteEvents(ctx context.Context, c changeSet, notes map[string]struct{}, timeout time.Duration,
	started time.Time, logger log.Logger) ([]event.Event, map[string]bool, error) {
	if len(c.commits) == 0 {
		return nil, nil, nil
	}

	var noteEvents []event.Event
	var eventTypes = make(map[string]bool)

	// Find notes in revisions.
	for i := len(c.commits) - 1; i >= 0; i-- {
		if _, ok := notes[c.commits[i].Revision]; !ok {
			eventTypes[event.NoneOfTheAbove] = true
			continue
		}
		var n note
		ctxGitOp, cancel := context.WithTimeout(ctx, timeout)
		ok, err := d.Repo.GetNote(ctxGitOp, c.commits[i].Revision, d.GitConfig.NotesRef, &n)
		cancel()
		if err != nil {
			return nil, nil, errors.Wrap(err, "loading notes from repo")
		}
		if !ok {
			eventTypes[event.NoneOfTheAbove] = true
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
		if c.initialSync {
			logger.Log("warning", "no notes expected on initial sync; this repo may be in use by another fluxd")
			return noteEvents, eventTypes, nil
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
						Revision: c.commits[i].Revision,
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
			eventTypes[event.EventRelease] = true
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
						Revision: c.commits[i].Revision,
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
			eventTypes[event.EventRelease] = true
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
						Revision: c.commits[i].Revision,
						Result:   n.Result,
						Error:    n.Result.Error(),
					},
					Spec: spec,
				},
			})
			eventTypes[event.EventAutoRelease] = true
		case update.Policy:
			// Use this to mean any change to policy
			eventTypes[event.EventUpdatePolicy] = true
		default:
			// Presume it's not something we're otherwise sending
			// as an event
			eventTypes[event.NoneOfTheAbove] = true
		}
	}
	return noteEvents, eventTypes, nil
}

// logCommitEvent reports all synced commits to the upstream.
func logCommitEvent(el eventLogger, c changeSet, serviceIDs resource.IDSet, started time.Time,
	includesEvents map[string]bool, resourceErrors []event.ResourceError, logger log.Logger) error {
	if len(c.commits) == 0 {
		return nil
	}
	cs := make([]event.Commit, len(c.commits))
	for i, ci := range c.commits {
		cs[i].Revision = ci.Revision
		cs[i].Message = ci.Message
	}
	if err := el.LogEvent(event.Event{
		ServiceIDs: serviceIDs.ToSlice(),
		Type:       event.EventSync,
		StartedAt:  started,
		EndedAt:    started,
		LogLevel:   event.LogLevelInfo,
		Metadata: &event.SyncEventMetadata{
			Commits:     cs,
			InitialSync: c.initialSync,
			Includes:    includesEvents,
			Errors:      resourceErrors,
		},
	}); err != nil {
		logger.Log("err", err)
		return err
	}
	return nil
}

// refresh refreshes the repository, notifying the daemon we have a new
// sync head.
func refresh(ctx context.Context, timeout time.Duration, repo *git.Repo) error {
	ctxGitOp, cancel := context.WithTimeout(ctx, timeout)
	err := repo.Refresh(ctxGitOp)
	cancel()
	return err
}

func makeGitConfigHash(remote git.Remote, conf git.Config) string {
	urlbit := remote.SafeURL()
	pathshash := sha256.New()
	pathshash.Write([]byte(urlbit))
	pathshash.Write([]byte(conf.Branch))
	for _, path := range conf.Paths {
		pathshash.Write([]byte(path))
	}
	return base64.RawURLEncoding.EncodeToString(pathshash.Sum(nil))
}
