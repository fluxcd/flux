package daemon

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	fluxerr "github.com/weaveworks/flux/errors"
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/guid"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/registry"
	"github.com/weaveworks/flux/release"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/update"
)

const (
	// This is set to be in sympathy with the request / RPC timeout (i.e., empirically)
	defaultHandlerTimeout = 10 * time.Second
	// A job can take an arbitrary amount of time but we want to have
	// a (generous) threshold for considering a job stuck and
	// abandoning it
	defaultJobTimeout = 60 * time.Second
)

// Daemon is the fully-functional state of a daemon (compare to
// `NotReadyDaemon`).
type Daemon struct {
	V              string
	Cluster        cluster.Cluster
	Manifests      cluster.Manifests
	Registry       registry.Registry
	ImageRefresh   chan image.Name
	Repo           git.Repo
	Checkout       *git.Checkout
	Jobs           *job.Queue
	JobStatusCache *job.StatusCache
	EventWriter    event.EventWriter
	Logger         log.Logger
	// bookkeeping
	*LoopVars
}

// Invariant.
var _ remote.Platform = &Daemon{}

func (d *Daemon) Version(ctx context.Context) (string, error) {
	return d.V, nil
}

func (d *Daemon) Ping(ctx context.Context) error {
	return d.Cluster.Ping()
}

func (d *Daemon) Export(ctx context.Context) ([]byte, error) {
	return d.Cluster.Export()
}

func (d *Daemon) ListServices(ctx context.Context, namespace string) ([]flux.ControllerStatus, error) {
	clusterServices, err := d.Cluster.AllControllers(namespace)
	if err != nil {
		return nil, errors.Wrap(err, "getting services from cluster")
	}

	d.Checkout.RLock()
	defer d.Checkout.RUnlock()

	services, err := d.Manifests.ServicesWithPolicies(d.Checkout.ManifestDir())
	if err != nil {
		return nil, errors.Wrap(err, "getting service policies")
	}

	var res []flux.ControllerStatus
	for _, service := range clusterServices {
		policies := services[service.ID]
		res = append(res, flux.ControllerStatus{
			ID:         service.ID,
			Containers: containers2containers(service.ContainersOrNil()),
			Status:     service.Status,
			Automated:  policies.Contains(policy.Automated),
			Locked:     policies.Contains(policy.Locked),
			Ignore:     policies.Contains(policy.Ignore),
			Policies:   policies.ToStringMap(),
		})
	}

	return res, nil
}

// List the images available for set of services
func (d *Daemon) ListImages(ctx context.Context, spec update.ResourceSpec) ([]flux.ImageStatus, error) {
	var services []cluster.Controller
	var err error
	if spec == update.ResourceSpecAll {
		services, err = d.Cluster.AllControllers("")
	} else {
		id, err := spec.AsID()
		if err != nil {
			return nil, errors.Wrap(err, "treating service spec as ID")
		}
		services, err = d.Cluster.SomeControllers([]flux.ResourceID{id})
	}

	images, err := update.CollectAvailableImages(d.Registry, services, d.Logger)
	if err != nil {
		return nil, errors.Wrap(err, "getting images for services")
	}

	var res []flux.ImageStatus
	for _, service := range services {
		containers := containersWithAvailable(service, images)
		res = append(res, flux.ImageStatus{
			ID:         service.ID,
			Containers: containers,
		})
	}

	return res, nil
}

// Let's use the CommitEventMetadata as a convenient transport for the
// results of a job; if no commit was made (e.g., if it was a dry
// run), leave the revision field empty.
type DaemonJobFunc func(ctx context.Context, jobID job.ID, working *git.Checkout, logger log.Logger) (*event.CommitEventMetadata, error)

// executeJob runs a job func in a cloned working directory, keeping track of its status.
func (d *Daemon) executeJob(id job.ID, do DaemonJobFunc, logger log.Logger) (*event.CommitEventMetadata, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultJobTimeout)
	defer cancel()
	d.JobStatusCache.SetStatus(id, job.Status{StatusString: job.StatusRunning})
	// make a working clone so we don't mess with files we
	// will be reading from elsewhere
	working, err := d.Checkout.WorkingClone(ctx)
	if err != nil {
		d.JobStatusCache.SetStatus(id, job.Status{StatusString: job.StatusFailed, Err: err.Error()})
		return nil, err
	}
	defer working.Clean()
	metadata, err := do(ctx, id, working, logger)
	if err != nil {
		d.JobStatusCache.SetStatus(id, job.Status{StatusString: job.StatusFailed, Err: err.Error()})
		return metadata, err
	}
	d.JobStatusCache.SetStatus(id, job.Status{StatusString: job.StatusSucceeded, Result: *metadata})
	return metadata, nil
}

// queueJob queues a job func to be executed.
func (d *Daemon) queueJob(do DaemonJobFunc) job.ID {
	id := job.ID(guid.New())
	enqueuedAt := time.Now()
	d.Jobs.Enqueue(&job.Job{
		ID: id,
		Do: func(logger log.Logger) error {
			queueDuration.Observe(time.Since(enqueuedAt).Seconds())
			started := time.Now().UTC()
			metadata, err := d.executeJob(id, do, logger)
			if err != nil {
				return err
			}
			logger.Log("revision", metadata.Revision)
			if metadata.Revision != "" {
				var serviceIDs []flux.ResourceID
				for id, result := range metadata.Result {
					if result.Status == update.ReleaseStatusSuccess {
						serviceIDs = append(serviceIDs, id)
					}
				}
				return d.LogEvent(event.Event{
					ServiceIDs: serviceIDs,
					Type:       event.EventCommit,
					StartedAt:  started,
					EndedAt:    started,
					LogLevel:   event.LogLevelInfo,
					Metadata:   metadata,
				})
			}
			return nil
		},
	})
	queueLength.Set(float64(d.Jobs.Len()))
	d.JobStatusCache.SetStatus(id, job.Status{StatusString: job.StatusQueued})
	return id
}

// Apply the desired changes to the config files
func (d *Daemon) UpdateManifests(ctx context.Context, spec update.Spec) (job.ID, error) {
	var id job.ID
	if spec.Type == "" {
		return id, errors.New("no type in update spec")
	}
	switch s := spec.Spec.(type) {
	case release.Changes:
		if s.ReleaseKind() == update.ReleaseKindPlan {
			id := job.ID(guid.New())
			_, err := d.executeJob(id, d.release(spec, s), d.Logger)
			return id, err
		}
		return d.queueJob(d.release(spec, s)), nil
	case policy.Updates:
		return d.queueJob(d.updatePolicy(spec, s)), nil
	default:
		return id, fmt.Errorf(`unknown update type "%s"`, spec.Type)
	}
}

func (d *Daemon) updatePolicy(spec update.Spec, updates policy.Updates) DaemonJobFunc {
	return func(ctx context.Context, jobID job.ID, working *git.Checkout, logger log.Logger) (*event.CommitEventMetadata, error) {
		// For each update
		var serviceIDs []flux.ResourceID
		metadata := &event.CommitEventMetadata{
			Spec:   &spec,
			Result: update.Result{},
		}

		// A shortcut to make things more responsive: if anything
		// was (probably) set to automated, we will ask for an
		// automation run straight ASAP.
		var anythingAutomated bool

		for serviceID, u := range updates {
			if policy.Set(u.Add).Contains(policy.Automated) {
				anythingAutomated = true
			}
			// find the service manifest
			err := cluster.UpdateManifest(d.Manifests, working.ManifestDir(), serviceID, func(def []byte) ([]byte, error) {
				newDef, err := d.Manifests.UpdatePolicies(def, serviceID, u)
				if err != nil {
					metadata.Result[serviceID] = update.ControllerResult{
						Status: update.ReleaseStatusFailed,
						Error:  err.Error(),
					}
					return nil, err
				}
				if string(newDef) == string(def) {
					metadata.Result[serviceID] = update.ControllerResult{
						Status: update.ReleaseStatusSkipped,
					}
				} else {
					serviceIDs = append(serviceIDs, serviceID)
					metadata.Result[serviceID] = update.ControllerResult{
						Status: update.ReleaseStatusSuccess,
					}
				}
				return newDef, nil
			})
			switch err {
			case cluster.ErrNoResourceFilesFoundForService, cluster.ErrMultipleResourceFilesFoundForService:
				metadata.Result[serviceID] = update.ControllerResult{
					Status: update.ReleaseStatusFailed,
					Error:  err.Error(),
				}
			case nil:
				// continue
			default:
				return nil, err
			}
		}
		if len(serviceIDs) == 0 {
			return metadata, nil
		}

		commitAuthor := ""
		if d.Checkout.Config.SetAuthor {
			commitAuthor = spec.Cause.User
		}
		commitAction := &git.CommitAction{Author: commitAuthor, Message: policyCommitMessage(updates, spec.Cause)}
		if err := working.CommitAndPush(ctx, commitAction, &git.Note{JobID: jobID, Spec: spec}); err != nil {
			// On the chance pushing failed because it was not
			// possible to fast-forward, ask for a sync so the
			// next attempt is more likely to succeed.
			d.AskForSync()
			return nil, err
		}
		if anythingAutomated {
			d.AskForImagePoll()
		}

		var err error
		metadata.Revision, err = working.HeadRevision(ctx)
		if err != nil {
			return nil, err
		}
		return metadata, nil
	}
}

func (d *Daemon) release(spec update.Spec, c release.Changes) DaemonJobFunc {
	return func(ctx context.Context, jobID job.ID, working *git.Checkout, logger log.Logger) (*event.CommitEventMetadata, error) {
		rc := release.NewReleaseContext(d.Cluster, d.Manifests, d.Registry, working)
		result, err := release.Release(rc, c, logger)
		if err != nil {
			return nil, err
		}

		var revision string
		if c.ReleaseKind() == update.ReleaseKindExecute {
			commitMsg := spec.Cause.Message
			if commitMsg == "" {
				commitMsg = c.CommitMessage()
			}
			commitAuthor := ""
			if d.Checkout.Config.SetAuthor {
				commitAuthor = spec.Cause.User
			}
			commitAction := &git.CommitAction{Author: commitAuthor, Message: commitMsg}
			if err := working.CommitAndPush(ctx, commitAction, &git.Note{JobID: jobID, Spec: spec, Result: result}); err != nil {
				// On the chance pushing failed because it was not
				// possible to fast-forward, ask for a sync so the
				// next attempt is more likely to succeed.
				d.AskForSync()
				return nil, err
			}
			revision, err = working.HeadRevision(ctx)
			if err != nil {
				return nil, err
			}
		}
		return &event.CommitEventMetadata{
			Revision: revision,
			Spec:     &spec,
			Result:   result,
		}, nil
	}
}

// Tell the daemon to synchronise the cluster with the manifests in
// the git repo. This has an error return value because upstream there
// may be comms difficulties or other sources of problems; here, we
// always succeed because it's just bookkeeping.
func (d *Daemon) NotifyChange(ctx context.Context, change remote.Change) error {
	switch change.Kind {
	case remote.GitChange:
		// TODO: check if it's actually our repo
		d.AskForSync()
	case remote.ImageChange:
		if imageUp, ok := change.Source.(remote.ImageUpdate); ok {
			if d.ImageRefresh != nil {
				d.ImageRefresh <- imageUp.Name
			}
		}
	}
	return nil
}

// JobStatus - Ask the daemon how far it's got committing things; in particular, is the job
// queued? running? committed? If it is done, the commit ref is returned.
func (d *Daemon) JobStatus(ctx context.Context, jobID job.ID) (job.Status, error) {
	// Is the job queued, running, or recently finished?
	status, ok := d.JobStatusCache.Status(jobID)
	if ok {
		return status, nil
	}

	// Look through the commits for a note referencing this job.  This
	// means that even if fluxd restarts, we will at least remember
	// jobs which have pushed a commit.
	notes, err := d.Checkout.NoteRevList(ctx)
	if err != nil {
		return job.Status{}, errors.Wrap(err, "enumerating commit notes")
	}
	commits, err := d.Checkout.CommitsBefore(ctx, "HEAD")
	if err != nil {
		return job.Status{}, errors.Wrap(err, "checking revisions for status")
	}

	for _, commit := range commits {
		if _, ok := notes[commit.Revision]; ok {
			note, _ := d.Checkout.GetNote(ctx, commit.Revision)
			if note != nil && note.JobID == jobID {
				return job.Status{
					StatusString: job.StatusSucceeded,
					Result: event.CommitEventMetadata{
						Revision: commit.Revision,
						Spec:     &note.Spec,
						Result:   note.Result,
					},
				}, nil
			}
		}
	}

	return job.Status{}, unknownJobError(jobID)
}

// Ask the daemon how far it's got applying things; in particular, is it
// past the supplied release? Return the list of commits between where
// we have applied and the ref given, inclusive. E.g., if you send HEAD,
// you'll get all the commits yet to be applied. If you send a hash
// and it's applied _past_ it, you'll get an empty list.
func (d *Daemon) SyncStatus(ctx context.Context, commitRef string) ([]string, error) {
	commits, err := d.Checkout.CommitsBetween(ctx, d.Checkout.SyncTag, commitRef)
	if err != nil {
		return nil, err
	}
	// NB we could use the messages too if we decide to change the
	// signature of the API to include it.
	revs := make([]string, len(commits))
	for i, commit := range commits {
		revs[i] = commit.Revision
	}
	return revs, nil
}

func (d *Daemon) GitRepoConfig(ctx context.Context, regenerate bool) (flux.GitConfig, error) {
	publicSSHKey, err := d.Cluster.PublicSSHKey(regenerate)
	if err != nil {
		return flux.GitConfig{}, err
	}
	return flux.GitConfig{
		Remote:       d.Repo.GitRemoteConfig,
		PublicSSHKey: publicSSHKey,
		Status:       flux.RepoReady,
	}, nil
}

// Non-remote.Platform methods

func unknownJobError(id job.ID) error {
	return &fluxerr.Error{
		Type: fluxerr.Missing,
		Err:  fmt.Errorf("unknown job %q", string(id)),
		Help: `Job not found

This is often because the job did not result in committing changes,
and therefore had no lasting effect. A release dry-run is an example
of a job that does not result in a commit.

If you were expecting changes to be committed, this may mean that the
job failed, but its status was lost.

In both of the above cases it is OK to retry the operation that
resulted in this error.

If you get this error repeatedly, it's probably a bug. Please log an
issue describing what you were attempting, and posting logs from the
daemon if possible:

    https://github.com/weaveworks/flux/issues

`,
	}
}

func (d *Daemon) LogEvent(ev event.Event) error {
	if d.EventWriter == nil {
		d.Logger.Log("event", ev, "logupstream", "false")
		return nil
	}
	d.Logger.Log("event", ev, "logupstream", "true")
	return d.EventWriter.LogEvent(ev)
}

// vvv helpers vvv

func containers2containers(cs []cluster.Container) []flux.Container {
	res := make([]flux.Container, len(cs))
	for i, c := range cs {
		id, _ := image.ParseRef(c.Image)
		res[i] = flux.Container{
			Name: c.Name,
			Current: image.Info{
				ID: id,
			},
		}
	}
	return res
}

func containersWithAvailable(service cluster.Controller, images update.ImageMap) (res []flux.Container) {
	for _, c := range service.ContainersOrNil() {
		im, _ := image.ParseRef(c.Image)
		available := images.Available(im.Name)
		res = append(res, flux.Container{
			Name: c.Name,
			Current: image.Info{
				ID: im,
			},
			Available: available,
		})
	}
	return res
}

func policyCommitMessage(us policy.Updates, cause update.Cause) string {
	// shortcut, since we want roughly the same information
	events := policyEvents(us, time.Now())
	commitMsg := &bytes.Buffer{}
	prefix := "- "
	switch {
	case cause.Message != "":
		fmt.Fprintf(commitMsg, "%s\n\n", cause.Message)
	case len(events) > 1:
		fmt.Fprintf(commitMsg, "Updated service policies\n\n")
	default:
		prefix = ""
	}

	for _, event := range events {
		fmt.Fprintf(commitMsg, "%s%v\n", prefix, event)
	}
	return commitMsg.String()
}

// policyEvents builds a map of events (by type), for all the events in this set of
// updates. There will be one event per type, containing all service ids
// affected by that event. e.g. all automated services will share an event.
func policyEvents(us policy.Updates, now time.Time) map[string]event.Event {
	eventsByType := map[string]event.Event{}
	for serviceID, update := range us {
		for _, eventType := range policyEventTypes(update) {
			e, ok := eventsByType[eventType]
			if !ok {
				e = event.Event{
					ServiceIDs: []flux.ResourceID{},
					Type:       eventType,
					StartedAt:  now,
					EndedAt:    now,
					LogLevel:   event.LogLevelInfo,
				}
			}
			e.ServiceIDs = append(e.ServiceIDs, serviceID)
			eventsByType[eventType] = e
		}
	}
	return eventsByType
}

// policyEventTypes is a deduped list of all event types this update contains
func policyEventTypes(u policy.Update) []string {
	types := map[string]struct{}{}
	for p, _ := range u.Add {
		switch {
		case p == policy.Automated:
			types[event.EventAutomate] = struct{}{}
		case p == policy.Locked:
			types[event.EventLock] = struct{}{}
		default:
			types[event.EventUpdatePolicy] = struct{}{}
		}
	}

	for p, _ := range u.Remove {
		switch {
		case p == policy.Automated:
			types[event.EventDeautomate] = struct{}{}
		case p == policy.Locked:
			types[event.EventUnlock] = struct{}{}
		default:
			types[event.EventUpdatePolicy] = struct{}{}
		}
	}
	var result []string
	for t := range types {
		result = append(result, t)
	}
	sort.Strings(result)
	return result
}
