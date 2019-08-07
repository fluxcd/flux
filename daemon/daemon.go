package daemon

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/api/v10"
	"github.com/weaveworks/flux/api/v11"
	"github.com/weaveworks/flux/api/v6"
	"github.com/weaveworks/flux/api/v9"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/guid"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/manifests"
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
	V                         string
	Cluster                   cluster.Cluster
	Manifests                 manifests.Manifests
	Registry                  registry.Registry
	ImageRefresh              chan image.Name
	Repo                      *git.Repo
	GitConfig                 git.Config
	Jobs                      *job.Queue
	JobStatusCache            *job.StatusCache
	EventWriter               event.EventWriter
	Logger                    log.Logger
	ManifestGenerationEnabled bool
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
	return d.Cluster.Export(ctx)
}

func (d *Daemon) getManifestStore(checkout *git.Checkout) (manifests.Store, error) {
	if d.ManifestGenerationEnabled {
		return manifests.NewConfigAware(checkout.Dir(), checkout.ManifestDirs(), d.Manifests)
	}
	return manifests.NewRawFiles(checkout.Dir(), checkout.ManifestDirs(), d.Manifests), nil
}

func (d *Daemon) getResources(ctx context.Context) (map[string]resource.Resource, v6.ReadOnlyReason, error) {
	var resources map[string]resource.Resource
	var globalReadOnly v6.ReadOnlyReason
	err := d.WithClone(ctx, func(checkout *git.Checkout) error {
		cm, err := d.getManifestStore(checkout)
		if err != nil {
			return err
		}
		resources, err = cm.GetAllResourcesByID(ctx)
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

	return resources, globalReadOnly, nil
}

func (d *Daemon) ListServices(ctx context.Context, namespace string) ([]v6.ControllerStatus, error) {
	return d.ListServicesWithOptions(ctx, v11.ListServicesOptions{Namespace: namespace})
}

func (d *Daemon) ListServicesWithOptions(ctx context.Context, opts v11.ListServicesOptions) ([]v6.ControllerStatus, error) {
	if opts.Namespace != "" && len(opts.Services) > 0 {
		return nil, errors.New("cannot filter by 'namespace' and 'workloads' at the same time")
	}

	var clusterWorkloads []cluster.Workload
	var err error
	if len(opts.Services) > 0 {
		clusterWorkloads, err = d.Cluster.SomeWorkloads(ctx, opts.Services)
	} else {
		clusterWorkloads, err = d.Cluster.AllWorkloads(ctx, opts.Namespace)
	}
	if err != nil {
		return nil, errors.Wrap(err, "getting workloads from cluster")
	}

	resources, missingReason, err := d.getResources(ctx)
	if err != nil {
		return nil, err
	}

	var res []v6.ControllerStatus
	for _, workload := range clusterWorkloads {
		readOnly := v6.ReadOnlyOK
		var policies policy.Set
		if resource, ok := resources[workload.ID.String()]; ok {
			policies = resource.Policies()
		}
		switch {
		case policies == nil:
			readOnly = missingReason
		case workload.IsSystem:
			readOnly = v6.ReadOnlySystem
		}
		var syncError string
		if workload.SyncError != nil {
			syncError = workload.SyncError.Error()
		}
		res = append(res, v6.ControllerStatus{
			ID:         workload.ID,
			Containers: containers2containers(workload.ContainersOrNil()),
			ReadOnly:   readOnly,
			Status:     workload.Status,
			Rollout:    workload.Rollout,
			SyncError:  syncError,
			Antecedent: workload.Antecedent,
			Labels:     workload.Labels,
			Automated:  policies.Has(policy.Automated),
			Locked:     policies.Has(policy.Locked),
			Ignore:     policies.Has(policy.Ignore),
			Policies:   policies.ToStringMap(),
		})
	}

	return res, nil
}

type clusterContainers []cluster.Workload

func (cs clusterContainers) Len() int {
	return len(cs)
}

func (cs clusterContainers) Containers(i int) []resource.Container {
	return cs[i].ContainersOrNil()
}

// ListImages - deprecated from v10, lists the images available for set of workloads
func (d *Daemon) ListImages(ctx context.Context, spec update.ResourceSpec) ([]v6.ImageStatus, error) {
	return d.ListImagesWithOptions(ctx, v10.ListImagesOptions{Spec: spec})
}

// ListImagesWithOptions lists the images available for set of workloads
func (d *Daemon) ListImagesWithOptions(ctx context.Context, opts v10.ListImagesOptions) ([]v6.ImageStatus, error) {
	if opts.Namespace != "" && opts.Spec != update.ResourceSpecAll {
		return nil, errors.New("cannot filter by 'namespace' and 'workload' at the same time")
	}

	var workloads []cluster.Workload
	var err error
	if opts.Spec != update.ResourceSpecAll {
		id, err := opts.Spec.AsID()
		if err != nil {
			return nil, errors.Wrap(err, "treating workload spec as ID")
		}
		workloads, err = d.Cluster.SomeWorkloads(ctx, []resource.ID{id})
		if err != nil {
			return nil, errors.Wrap(err, "getting some workloads")
		}
	} else {
		workloads, err = d.Cluster.AllWorkloads(ctx, opts.Namespace)
		if err != nil {
			return nil, errors.Wrap(err, "getting all workloads")
		}
	}

	resources, _, err := d.getResources(ctx)
	if err != nil {
		return nil, err
	}

	imageRepos, err := update.FetchImageRepos(d.Registry, clusterContainers(workloads), d.Logger)
	if err != nil {
		return nil, errors.Wrap(err, "getting images for workloads")
	}

	var res []v6.ImageStatus
	for _, workload := range workloads {
		workloadContainers, err := getWorkloadContainers(workload, imageRepos, resources[workload.ID.String()], opts.OverrideContainerFields)
		if err != nil {
			return nil, err
		}
		res = append(res, v6.ImageStatus{
			ID:         workload.ID,
			Containers: workloadContainers,
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
			if err = verifyWorkingRepo(ctx, d.Repo, working, d.GitConfig); d.GitVerifySignatures && err != nil {
				return err
			}
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
		d.JobStatusCache.SetStatus(id, job.Status{StatusString: job.StatusFailed, Err: err.Error(), Result: result})
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
			var workloadIDs []resource.ID
			for id, result := range result.Result {
				if result.Status == update.ReleaseStatusSuccess {
					workloadIDs = append(workloadIDs, id)
				}
			}

			metadata := &event.CommitEventMetadata{
				Revision: result.Revision,
				Spec:     result.Spec,
				Result:   result.Result,
			}

			return result, d.LogEvent(event.Event{
				ServiceIDs: workloadIDs,
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
	case resource.PolicyUpdates:
		return d.queueJob(d.makeLoggingJobFunc(d.makeJobFromUpdate(d.updatePolicies(spec, s)))), nil
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
		head, err := d.Repo.BranchHead(ctx)
		if err != nil {
			return result, err
		}
		if d.GitVerifySignatures {
			var latestValidRev string
			if latestValidRev, _, err = latestValidRevision(ctx, d.Repo, d.GitConfig); err != nil {
				return result, err
			} else if head != latestValidRev {
				result.Revision = latestValidRev
				return result, fmt.Errorf(
					"The branch HEAD in the git repo is not verified, and fluxd is unable to sync to it. The last verified commit was %.8s. HEAD is %.8s.",
					latestValidRev,
					head,
				)
			}
		}
		result.Revision = head
		return result, err
	}
}

func (d *Daemon) updatePolicies(spec update.Spec, updates resource.PolicyUpdates) updateFunc {
	return func(ctx context.Context, jobID job.ID, working *git.Checkout, logger log.Logger) (job.Result, error) {
		// For each update
		var workloadIDs []resource.ID
		result := job.Result{
			Spec:   &spec,
			Result: update.Result{},
		}

		// A shortcut to make things more responsive: if anything
		// was (probably) set to automated, we will ask for an
		// automation run straight ASAP.
		var anythingAutomated bool

		for workloadID, u := range updates {
			if d.Cluster.IsAllowedResource(workloadID) {
				result.Result[workloadID] = update.WorkloadResult{
					Status: update.ReleaseStatusSkipped,
				}
			}
			if policy.Set(u.Add).Has(policy.Automated) {
				anythingAutomated = true
			}
			cm, err := d.getManifestStore(working)
			if err != nil {
				return result, err
			}
			updated, err := cm.UpdateWorkloadPolicies(ctx, workloadID, u)
			if err != nil {
				result.Result[workloadID] = update.WorkloadResult{
					Status: update.ReleaseStatusFailed,
					Error:  err.Error(),
				}
				switch err := err.(type) {
				case manifests.StoreError:
					result.Result[workloadID] = update.WorkloadResult{
						Status: update.ReleaseStatusFailed,
						Error:  err.Error(),
					}
				default:
					return result, err
				}
			}
			if !updated {
				result.Result[workloadID] = update.WorkloadResult{
					Status: update.ReleaseStatusSkipped,
				}
			} else {
				workloadIDs = append(workloadIDs, workloadID)
				result.Result[workloadID] = update.WorkloadResult{
					Status: update.ReleaseStatusSuccess,
				}
			}
		}
		if len(workloadIDs) == 0 {
			return result, nil
		}

		commitAuthor := ""
		if d.GitConfig.SetAuthor {
			commitAuthor = spec.Cause.User
		}
		commitAction := git.CommitAction{
			Author:  commitAuthor,
			Message: policyCommitMessage(updates, spec.Cause),
		}
		if err := working.CommitAndPush(ctx, commitAction, &note{JobID: jobID, Spec: spec}, d.ManifestGenerationEnabled); err != nil {
			// On the chance pushing failed because it was not
			// possible to fast-forward, ask for a sync so the
			// next attempt is more likely to succeed.
			d.AskForSync()
			return result, err
		}
		if anythingAutomated {
			d.AskForAutomatedWorkloadImageUpdates()
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
		var zero job.Result
		rs, err := d.getManifestStore(working)
		if err != nil {
			return zero, err
		}
		rc := release.NewReleaseContext(d.Cluster, rs, d.Registry)
		result, err := release.Release(ctx, rc, c, logger)
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
			commitAction := git.CommitAction{
				Author:  commitAuthor,
				Message: commitMsg,
			}
			if err := working.CommitAndPush(ctx, commitAction, &note{JobID: jobID, Spec: spec, Result: result}, d.ManifestGenerationEnabled); err != nil {
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
		commits, err := d.Repo.CommitsBefore(ctx, "HEAD", d.GitConfig.Paths...)
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
	commits, err := d.Repo.CommitsBetween(ctx, d.GitConfig.SyncTag, commitRef, d.GitConfig.Paths...)
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
	path := ""
	if len(d.GitConfig.Paths) > 0 {
		path = strings.Join(d.GitConfig.Paths, ",")
	}
	return v6.GitConfig{
		Remote: v6.GitRemoteConfig{
			URL:    origin.URL,
			Branch: d.GitConfig.Branch,
			Path:   path,
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

func getWorkloadContainers(workload cluster.Workload, imageRepos update.ImageRepos, resource resource.Resource, fields []string) (res []v6.Container, err error) {
	for _, c := range workload.ContainersOrNil() {
		imageRepo := c.Image.Name
		var policies policy.Set
		if resource != nil {
			policies = resource.Policies()
		}
		tagPattern := policy.GetTagPattern(policies, c.Name)

		repoMetadata := imageRepos.GetRepositoryMetadata(imageRepo)
		var images []image.Info
		// Build images, tolerating tags with missing metadata
		for _, tag := range repoMetadata.Tags {
			info, ok := repoMetadata.Images[tag]
			if !ok {
				info = image.Info{
					ID: image.Ref{Tag: tag},
				}
			}
			images = append(images, info)
		}
		currentImage := repoMetadata.FindImageWithRef(c.Image)

		container, err := v6.NewContainer(c.Name, images, currentImage, tagPattern, fields)
		if err != nil {
			return res, err
		}
		res = append(res, container)
	}

	return res, nil
}

func policyCommitMessage(us resource.PolicyUpdates, cause update.Cause) string {
	// shortcut, since we want roughly the same information
	events := policyEvents(us, time.Now())
	commitMsg := &bytes.Buffer{}
	prefix := "- "
	switch {
	case cause.Message != "":
		fmt.Fprintf(commitMsg, "%s\n\n", cause.Message)
	case len(events) > 1:
		fmt.Fprintf(commitMsg, "Updated workload policies\n\n")
	default:
		prefix = ""
	}

	for _, event := range events {
		fmt.Fprintf(commitMsg, "%s%v\n", prefix, event)
	}
	return commitMsg.String()
}

// policyEvents builds a map of events (by type), for all the events in this set of
// updates. There will be one event per type, containing all workload ids
// affected by that event. e.g. all automated workload will share an event.
func policyEvents(us resource.PolicyUpdates, now time.Time) map[string]event.Event {
	eventsByType := map[string]event.Event{}
	for workloadID, update := range us {
		for _, eventType := range policyEventTypes(update) {
			e, ok := eventsByType[eventType]
			if !ok {
				e = event.Event{
					ServiceIDs: []resource.ID{},
					Type:       eventType,
					StartedAt:  now,
					EndedAt:    now,
					LogLevel:   event.LogLevelInfo,
				}
			}
			e.ServiceIDs = append(e.ServiceIDs, workloadID)
			eventsByType[eventType] = e
		}
	}
	return eventsByType
}

// policyEventTypes is a deduped list of all event types this update contains
func policyEventTypes(u resource.PolicyUpdate) []string {
	types := map[string]struct{}{}
	for p := range u.Add {
		switch {
		case p == policy.Automated:
			types[event.EventAutomate] = struct{}{}
		case p == policy.Locked:
			types[event.EventLock] = struct{}{}
		default:
			types[event.EventUpdatePolicy] = struct{}{}
		}
	}

	for p := range u.Remove {
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

// latestValidRevision returns the HEAD of the configured branch if it
// has a valid signature, or the SHA of the latest valid commit it
// could find plus the invalid commit thereafter.
//
// Signature validation happens for commits between the revision of the
// sync tag and the HEAD, after the signature of the sync tag itself
// has been validated, as the branch can not be trusted when the tag
// originates from an unknown source.
//
// In case the signature of the tag can not be verified, or it points
// towards a revision we can not get a commit range for, it returns an
// error.
func latestValidRevision(ctx context.Context, repo *git.Repo, gitConfig git.Config) (string, git.Commit, error) {
	var invalidCommit = git.Commit{}
	newRevision, err := repo.BranchHead(ctx)
	if err != nil {
		return "", invalidCommit, err
	}

	// Validate tag and retrieve the revision it points to
	tagRevision, err := repo.VerifyTag(ctx, gitConfig.SyncTag)
	if err != nil && !strings.Contains(err.Error(), "not found.") {
		return "", invalidCommit, errors.Wrap(err, "failed to verify signature of sync tag")
	}

	var commits []git.Commit
	if tagRevision == "" {
		commits, err = repo.CommitsBefore(ctx, newRevision)
	} else {
		// Assure the revision from the tag is a signed and valid commit
		if err = repo.VerifyCommit(ctx, tagRevision); err != nil {
			return "", invalidCommit, errors.Wrap(err, "failed to verify signature of sync tag revision")
		}
		commits, err = repo.CommitsBetween(ctx, tagRevision, newRevision)
	}

	if err != nil {
		return tagRevision, invalidCommit, err
	}

	// Loop through commits in ascending order, validating the
	// signature of each commit. In case we hit an invalid commit, we
	// return the revision of the commit before that, as that one is
	// valid.
	for i := len(commits) - 1; i >= 0; i-- {
		if !commits[i].Signature.Valid() {
			if i+1 < len(commits) {
				return commits[i+1].Revision, commits[i], nil
			}
			return tagRevision, commits[i], nil
		}
	}

	return newRevision, invalidCommit, nil
}

func verifyWorkingRepo(ctx context.Context, repo *git.Repo, working *git.Checkout, gitConfig git.Config) error {
	if latestVerifiedRev, _, err := latestValidRevision(ctx, repo, gitConfig); err != nil {
		return err
	} else if headRev, err := working.HeadRevision(ctx); err != nil {
		return err
	} else if headRev != latestVerifiedRev {
		return unsignedHeadRevisionError(latestVerifiedRev, headRev)
	}
	return nil
}

func isUnknownRevision(err error) bool {
	return err != nil &&
		(strings.Contains(err.Error(), "unknown revision or path not in the working tree.") ||
			strings.Contains(err.Error(), "bad revision"))
}
