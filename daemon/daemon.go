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
	"github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/api/v10"
	"github.com/weaveworks/flux/api/v6"
	"github.com/weaveworks/flux/api/v9"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/guid"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/registry"
	"github.com/weaveworks/flux/release"
	"github.com/weaveworks/flux/resource"
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
	Repo           *git.Repo
	GitConfig      git.Config
	Jobs           *job.Queue
	JobStatusCache *job.StatusCache
	EventWriter    event.EventWriter
	Logger         log.Logger
	// bookkeeping
	*LoopVars
}

// Invariant.
var _ api.Server = &Daemon{}

func (d *Daemon) Version(ctx context.Context) (string, error) {
	return d.V, nil
}

func (d *Daemon) Ping(ctx context.Context) error {
	return d.Cluster.Ping()
}

func (d *Daemon) Export(ctx context.Context) ([]byte, error) {
	return d.Cluster.Export()
}

func (d *Daemon) getPolicyResourceMap(ctx context.Context) (policy.ResourceMap, v6.ReadOnlyReason, error) {
	var services policy.ResourceMap
	var globalReadOnly v6.ReadOnlyReason
	err := d.WithClone(ctx, func(checkout *git.Checkout) error {
		var err error
		services, err = d.Manifests.ServicesWithPolicies(checkout.ManifestDir())
		return err
	})

	// The reason something is missing from the map differs depending
	// on the state of the git repo.
	_, notReady := err.(git.NotReadyError)
	switch {
	case notReady:
		globalReadOnly = v6.ReadOnlyNotReady
	case err == git.ErrNoConfig:
		globalReadOnly = v6.ReadOnlyNoRepo
	case err != nil:
		return nil, globalReadOnly, manifestLoadError(err)
	default:
		globalReadOnly = v6.ReadOnlyMissing
	}

	return services, globalReadOnly, nil
}

func (d *Daemon) ListServices(ctx context.Context, namespace string) ([]v6.ControllerStatus, error) {
	clusterServices, err := d.Cluster.AllControllers(namespace)
	if err != nil {
		return nil, errors.Wrap(err, "getting services from cluster")
	}

	policyResourceMap, missingReason, err := d.getPolicyResourceMap(ctx)
	if err != nil {
		return nil, err
	}

	var res []v6.ControllerStatus
	for _, service := range clusterServices {
		readOnly := v6.ReadOnlyOK
		policies, ok := policyResourceMap[service.ID]
		switch {
		case !ok:
			readOnly = missingReason
		case service.IsSystem:
			readOnly = v6.ReadOnlySystem
		}
		res = append(res, v6.ControllerStatus{
			ID:         service.ID,
			Containers: containers2containers(service.ContainersOrNil()),
			ReadOnly:   readOnly,
			Status:     service.Status,
			Antecedent: service.Antecedent,
			Labels:     service.Labels,
			Automated:  policies.Contains(policy.Automated),
			Locked:     policies.Contains(policy.Locked),
			Ignore:     policies.Contains(policy.Ignore),
			Policies:   policies.ToStringMap(),
		})
	}

	return res, nil
}

type clusterContainers []cluster.Controller

func (cs clusterContainers) Len() int {
	return len(cs)
}

func (cs clusterContainers) Containers(i int) []resource.Container {
	return cs[i].ContainersOrNil()
}

// ListImages - deprecated from v10, lists the images available for set of services
func (d *Daemon) ListImages(ctx context.Context, spec update.ResourceSpec) ([]v6.ImageStatus, error) {
	return d.ListImagesWithOptions(ctx, v10.ListImagesOptions{Spec: spec})
}

// ListImagesWithOptions lists the images available for set of services
func (d *Daemon) ListImagesWithOptions(ctx context.Context, opts v10.ListImagesOptions) ([]v6.ImageStatus, error) {
	var services []cluster.Controller
	var err error
	if opts.Spec == update.ResourceSpecAll {
		services, err = d.Cluster.AllControllers("")
	} else {
		id, err := opts.Spec.AsID()
		if err != nil {
			return nil, errors.Wrap(err, "treating service spec as ID")
		}
		services, err = d.Cluster.SomeControllers([]flux.ResourceID{id})
	}

	imageRepos, err := update.FetchImageRepos(d.Registry, clusterContainers(services), d.Logger)
	if err != nil {
		return nil, errors.Wrap(err, "getting images for services")
	}

	policyResourceMap, _, err := d.getPolicyResourceMap(ctx)
	if err != nil {
		return nil, err
	}

	var res []v6.ImageStatus
	for _, service := range services {
		serviceContainers, err := getServiceContainers(service, imageRepos, policyResourceMap, opts.OverrideContainerFields)
		if err != nil {
			return nil, err
		}
		res = append(res, v6.ImageStatus{
			ID:         service.ID,
			Containers: serviceContainers,
		})
	}

	return res, nil
}

// jobFunc is a type for procedures that the daemon will execute in a job
type jobFunc func(ctx context.Context, jobID job.ID, logger log.Logger) (job.Result, error)

// updateFunc is a type for procedures that operate on a git checkout, to be run in a job
type updateFunc func(ctx context.Context, jobID job.ID, working *git.Checkout, logger log.Logger) (job.Result, error)

// makeJobFromUpdate turns an updateFunc into a jobFunc that will run
// the update with a fresh clone, and log the result as an event.
func (d *Daemon) makeJobFromUpdate(update updateFunc) jobFunc {
	return func(ctx context.Context, jobID job.ID, logger log.Logger) (job.Result, error) {
		var result job.Result
		err := d.WithClone(ctx, func(working *git.Checkout) error {
			var err error
			result, err = update(ctx, jobID, working, logger)
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return result, err
		}
		return result, nil
	}
}

// executeJob runs a job func and keeps track of its status, so the
// daemon can report it when asked.
func (d *Daemon) executeJob(id job.ID, do jobFunc, logger log.Logger) (job.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultJobTimeout)
	defer cancel()
	d.JobStatusCache.SetStatus(id, job.Status{StatusString: job.StatusRunning})
	result, err := do(ctx, id, logger)
	if err != nil {
		d.JobStatusCache.SetStatus(id, job.Status{StatusString: job.StatusFailed, Err: err.Error()})
		return result, err
	}
	d.JobStatusCache.SetStatus(id, job.Status{StatusString: job.StatusSucceeded, Result: result})
	return result, nil
}

// makeLoggingFunc takes a jobFunc and returns a jobFunc that will log
// a commit event with the result.
func (d *Daemon) makeLoggingJobFunc(f jobFunc) jobFunc {
	return func(ctx context.Context, id job.ID, logger log.Logger) (job.Result, error) {
		started := time.Now().UTC()
		result, err := f(ctx, id, logger)
		if err != nil {
			return result, err
		}
		logger.Log("revision", result.Revision)
		if result.Revision != "" {
			var serviceIDs []flux.ResourceID
			for id, result := range result.Result {
				if result.Status == update.ReleaseStatusSuccess {
					serviceIDs = append(serviceIDs, id)
				}
			}

			metadata := &event.CommitEventMetadata{
				Revision: result.Revision,
				Spec:     result.Spec,
				Result:   result.Result,
			}

			return result, d.LogEvent(event.Event{
				ServiceIDs: serviceIDs,
				Type:       event.EventCommit,
				StartedAt:  started,
				EndedAt:    started,
				LogLevel:   event.LogLevelInfo,
				Metadata:   metadata,
			})
		}
		return result, nil
	}
}

// queueJob queues a job func to be executed.
func (d *Daemon) queueJob(do jobFunc) job.ID {
	id := job.ID(guid.New())
	enqueuedAt := time.Now()
	d.Jobs.Enqueue(&job.Job{
		ID: id,
		Do: func(logger log.Logger) error {
			queueDuration.Observe(time.Since(enqueuedAt).Seconds())
			_, err := d.executeJob(id, do, logger)
			if err != nil {
				return err
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
			_, err := d.executeJob(id, d.makeJobFromUpdate(d.release(spec, s)), d.Logger)
			return id, err
		}
		return d.queueJob(d.makeLoggingJobFunc(d.makeJobFromUpdate(d.release(spec, s)))), nil
	case policy.Updates:
		return d.queueJob(d.makeLoggingJobFunc(d.makeJobFromUpdate(d.updatePolicy(spec, s)))), nil
	case update.ManualSync:
		return d.queueJob(d.sync()), nil
	default:
		return id, fmt.Errorf(`unknown update type "%s"`, spec.Type)
	}
}

func (d *Daemon) sync() jobFunc {
	return func(ctx context.Context, jobID job.ID, logger log.Logger) (job.Result, error) {
		var result job.Result
		ctx, cancel := context.WithTimeout(ctx, defaultJobTimeout)
		defer cancel()
		err := d.Repo.Refresh(ctx)
		if err != nil {
			return result, err
		}
		head, err := d.Repo.Revision(ctx, d.GitConfig.Branch)
		if err != nil {
			return result, err
		}
		result.Revision = head
		return result, nil
	}
}

func (d *Daemon) updatePolicy(spec update.Spec, updates policy.Updates) updateFunc {
	return func(ctx context.Context, jobID job.ID, working *git.Checkout, logger log.Logger) (job.Result, error) {
		// For each update
		var serviceIDs []flux.ResourceID
		result := job.Result{
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
					result.Result[serviceID] = update.ControllerResult{
						Status: update.ReleaseStatusFailed,
						Error:  err.Error(),
					}
					return nil, err
				}
				if string(newDef) == string(def) {
					result.Result[serviceID] = update.ControllerResult{
						Status: update.ReleaseStatusSkipped,
					}
				} else {
					serviceIDs = append(serviceIDs, serviceID)
					result.Result[serviceID] = update.ControllerResult{
						Status: update.ReleaseStatusSuccess,
					}
				}
				return newDef, nil
			})
			if err != nil {
				switch err := err.(type) {
				case cluster.ManifestError:
					result.Result[serviceID] = update.ControllerResult{
						Status: update.ReleaseStatusFailed,
						Error:  err.Error(),
					}
				default:
					return result, err
				}
			}
		}
		if len(serviceIDs) == 0 {
			return result, nil
		}

		commitAuthor := ""
		if d.GitConfig.SetAuthor {
			commitAuthor = spec.Cause.User
		}
		commitAction := git.CommitAction{Author: commitAuthor, Message: policyCommitMessage(updates, spec.Cause)}
		if err := working.CommitAndPush(ctx, commitAction, &note{JobID: jobID, Spec: spec}); err != nil {
			// On the chance pushing failed because it was not
			// possible to fast-forward, ask for a sync so the
			// next attempt is more likely to succeed.
			d.AskForSync()
			return result, err
		}
		if anythingAutomated {
			d.AskForImagePoll()
		}

		var err error
		result.Revision, err = working.HeadRevision(ctx)
		if err != nil {
			return result, err
		}
		return result, nil
	}
}

func (d *Daemon) release(spec update.Spec, c release.Changes) updateFunc {
	return func(ctx context.Context, jobID job.ID, working *git.Checkout, logger log.Logger) (job.Result, error) {
		rc := release.NewReleaseContext(d.Cluster, d.Manifests, d.Registry, working)
		result, err := release.Release(rc, c, logger)

		var zero job.Result
		if err != nil {
			return zero, err
		}

		var revision string

		if c.ReleaseKind() == update.ReleaseKindExecute {
			commitMsg := spec.Cause.Message
			if commitMsg == "" {
				commitMsg = c.CommitMessage(result)
			}
			commitAuthor := ""
			if d.GitConfig.SetAuthor {
				commitAuthor = spec.Cause.User
			}
			commitAction := git.CommitAction{Author: commitAuthor, Message: commitMsg}
			if err := working.CommitAndPush(ctx, commitAction, &note{JobID: jobID, Spec: spec, Result: result}); err != nil {
				// On the chance pushing failed because it was not
				// possible to fast-forward, ask the repo to fetch
				// from upstream ASAP, so the next attempt is more
				// likely to succeed.
				d.Repo.Notify()
				return zero, err
			}
			revision, err = working.HeadRevision(ctx)
			if err != nil {
				return zero, err
			}
		}
		return job.Result{
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
func (d *Daemon) NotifyChange(ctx context.Context, change v9.Change) error {
	switch change.Kind {
	case v9.GitChange:
		gitUpdate := change.Source.(v9.GitUpdate)
		if gitUpdate.URL != d.Repo.Origin().URL && gitUpdate.Branch != d.GitConfig.Branch {
			// It isn't strictly an _error_ to be notified about a repo/branch pair
			// that isn't ours, but it's worth logging anyway for debugging.
			d.Logger.Log("msg", "notified about unrelated change",
				"url", gitUpdate.URL,
				"branch", gitUpdate.Branch)
			break
		}
		d.Repo.Notify()
	case v9.ImageChange:
		imageUpdate := change.Source.(v9.ImageUpdate)
		d.ImageRefresh <- imageUpdate.Name
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
	// FIXME(michael): consider looking at the repo for this, since read op
	err := d.WithClone(ctx, func(working *git.Checkout) error {
		notes, err := working.NoteRevList(ctx)
		if err != nil {
			return errors.Wrap(err, "enumerating commit notes")
		}
		commits, err := d.Repo.CommitsBefore(ctx, "HEAD", d.GitConfig.Path)
		if err != nil {
			return errors.Wrap(err, "checking revisions for status")
		}

		for _, commit := range commits {
			if _, ok := notes[commit.Revision]; ok {
				var n note
				ok, err := working.GetNote(ctx, commit.Revision, &n)
				if ok && err == nil && n.JobID == jobID {
					status = job.Status{
						StatusString: job.StatusSucceeded,
						Result: job.Result{
							Revision: commit.Revision,
							Spec:     &n.Spec,
							Result:   n.Result,
						},
					}
					return nil
				}
			}
		}
		return unknownJobError(jobID)
	})
	return status, err
}

// Ask the daemon how far it's got applying things; in particular, is it
// past the given commit? Return the list of commits between where
// we have applied (the sync tag) and the ref given, inclusive. E.g., if you send HEAD,
// you'll get all the commits yet to be applied. If you send a hash
// and it's applied at or _past_ it, you'll get an empty list.
func (d *Daemon) SyncStatus(ctx context.Context, commitRef string) ([]string, error) {
	commits, err := d.Repo.CommitsBetween(ctx, d.GitConfig.SyncTag, commitRef, d.GitConfig.Path)
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

func (d *Daemon) GitRepoConfig(ctx context.Context, regenerate bool) (v6.GitConfig, error) {
	publicSSHKey, err := d.Cluster.PublicSSHKey(regenerate)
	if err != nil {
		return v6.GitConfig{}, err
	}

	origin := d.Repo.Origin()
	status, _ := d.Repo.Status()
	return v6.GitConfig{
		Remote: v6.GitRemoteConfig{
			URL:    origin.URL,
			Branch: d.GitConfig.Branch,
			Path:   d.GitConfig.Path,
		},
		PublicSSHKey: publicSSHKey,
		Status:       status,
	}, nil
}

// Non-api.Server methods

func (d *Daemon) WithClone(ctx context.Context, fn func(*git.Checkout) error) error {
	co, err := d.Repo.Clone(ctx, d.GitConfig)
	if err != nil {
		return err
	}
	defer co.Clean()
	return fn(co)
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

func containers2containers(cs []resource.Container) []v6.Container {
	res := make([]v6.Container, len(cs))
	for i, c := range cs {
		res[i] = v6.Container{
			Name: c.Name,
			Current: image.Info{
				ID: c.Image,
			},
		}
	}
	return res
}

func getServiceContainers(service cluster.Controller, imageRepos update.ImageRepos, policyResourceMap policy.ResourceMap, fields []string) (res []v6.Container, err error) {
	for _, c := range service.ContainersOrNil() {
		imageRepo := c.Image.Name
		tagPattern := policy.GetTagPattern(policyResourceMap, service.ID, c.Name)

		images := imageRepos.GetRepoImages(imageRepo)
		currentImage := images.FindWithRef(c.Image)

		container, err := v6.NewContainer(c.Name, images, currentImage, tagPattern, fields)
		if err != nil {
			return res, err
		}
		res = append(res, container)
	}

	return res, nil
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
