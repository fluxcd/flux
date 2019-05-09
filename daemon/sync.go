package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"time"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/resource"
	fluxsync "github.com/weaveworks/flux/sync"
	"github.com/weaveworks/flux/update"
)

type syncTag interface {
	SetRevision(ctx context.Context, working *git.Checkout, timeout time.Duration, oldRev, newRev string) (bool, error)
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
func (d *Daemon) Sync(ctx context.Context, started time.Time, revision string, syncTag syncTag) error {
	// Checkout a working clone used for this sync
	ctxt, cancel := context.WithTimeout(ctx, d.GitTimeout)
	working, err := d.Repo.Clone(ctxt, d.GitConfig)
	if err != nil {
		return err
	}
	cancel()
	defer working.Clean()

	// Ensure we are syncing the given revision
	if err := working.Checkout(ctx, revision); err != nil {
		return err
	}

	// Retrieve change set of commits we need to sync
	c, err := getChangeSet(ctx, working, d.Repo, d.GitTimeout, d.GitConfig.Paths)
	if err != nil {
		return err
	}

	// Run actual sync of resources on cluster
	syncSetName := makeGitConfigHash(d.Repo.Origin(), d.GitConfig)
	resources, resourceErrors, err := doSync(d.Manifests, working, d.Cluster, syncSetName, d.Logger)
	if err != nil {
		return err
	}

	// Determine what resources changed during the sync
	changedResources, err := getChangedResources(ctx, c, d.GitTimeout, working, d.Manifests, resources)
	serviceIDs := flux.ResourceIDSet{}
	for _, r := range changedResources {
		serviceIDs.Add([]flux.ResourceID{r.ResourceID()})
	}

	// Retrieve git notes and collect events from them
	notes, err := getNotes(ctx, d.GitTimeout, working)
	if err != nil {
		return err
	}
	noteEvents, includesEvents, err := collectNoteEvents(ctx, c, notes, d.GitTimeout, working, started, d.Logger)
	if err != nil {
		return err
	}

	// Report all synced commits
	if err := logCommitEvent(d, c, serviceIDs, started, includesEvents, resourceErrors, d.Logger); err != nil {
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

	// Move sync tag
	if ok, err := syncTag.SetRevision(ctx, working, d.GitTimeout, c.oldTagRev, c.newTagRev); err != nil {
		return err
	} else if !ok {
		return nil
	}

	err = refresh(ctx, d.GitTimeout, d.Repo)
	return err
}

// getChangeSet returns the change set of commits for this sync,
// including the revision range and if it is an initial sync.
func getChangeSet(ctx context.Context, working *git.Checkout, repo *git.Repo, timeout time.Duration,
	paths []string) (changeSet, error) {
	var c changeSet
	var err error

	c.oldTagRev, err = working.SyncRevision(ctx)
	if err != nil && !isUnknownRevision(err) {
		return c, err
	}
	c.newTagRev, err = working.HeadRevision(ctx)
	if err != nil {
		return c, err
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	if c.oldTagRev != "" {
		c.commits, err = repo.CommitsBetween(ctx, c.oldTagRev, c.newTagRev, paths...)
	} else {
		c.initialSync = true
		c.commits, err = repo.CommitsBefore(ctx, c.newTagRev, paths...)
	}
	cancel()

	return c, err
}

// doSync runs the actual sync of workloads on the cluster. It returns
// a map with all resources it applied and sync errors it encountered.
func doSync(manifests cluster.Manifests, working *git.Checkout, clus cluster.Cluster, syncSetName string,
	logger log.Logger) (map[string]resource.Resource, []event.ResourceError, error) {
	resources, err := manifests.LoadManifests(working.Dir(), working.ManifestDirs())
	if err != nil {
		return nil, nil, errors.Wrap(err, "loading resources from repo")
	}

	var resourceErrors []event.ResourceError
	if err := fluxsync.Sync(syncSetName, resources, clus); err != nil {
		switch syncerr := err.(type) {
		case cluster.SyncError:
			logger.Log("err", err)
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
	}
	return resources, resourceErrors, nil
}

// getChangedResources calculates what resources are modified during
// this sync.
func getChangedResources(ctx context.Context, c changeSet, timeout time.Duration, working *git.Checkout,
	manifests cluster.Manifests, resources map[string]resource.Resource) (map[string]resource.Resource, error) {
	if c.initialSync {
		return resources, nil
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	changedFiles, err := working.ChangedFiles(ctx, c.oldTagRev)
	if err == nil && len(changedFiles) > 0 {
		// We had some changed files, we're syncing a diff
		// FIXME(michael): this won't be accurate when a file can have more than one resource
		resources, err = manifests.LoadManifests(working.Dir(), changedFiles)
	}
	cancel()
	if err != nil {
		return nil, errors.Wrap(err, "loading resources from repo")
	}
	return resources, nil
}

// getNotes retrieves the git notes from the working clone.
func getNotes(ctx context.Context, timeout time.Duration, working *git.Checkout) (map[string]struct{}, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	notes, err := working.NoteRevList(ctx)
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
func collectNoteEvents(ctx context.Context, c changeSet, notes map[string]struct{}, timeout time.Duration,
	working *git.Checkout, started time.Time, logger log.Logger) ([]event.Event, map[string]bool, error) {
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
		ctx, cancel := context.WithTimeout(ctx, timeout)
		ok, err := working.GetNote(ctx, c.commits[i].Revision, &n)
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
func logCommitEvent(el eventLogger, c changeSet, serviceIDs flux.ResourceIDSet, started time.Time,
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
	ctx, cancel := context.WithTimeout(ctx, timeout)
	err := repo.Refresh(ctx)
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
