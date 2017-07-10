package daemon

import (
	"bytes"
	"fmt"
	"sort"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/guid"
	"github.com/weaveworks/flux/history"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/registry"
	"github.com/weaveworks/flux/release"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/update"
)

var ErrUnknownJob = fmt.Errorf("unknown job")

// Combine these things to form Devasta^Wan implementation of
// Platform.
type Daemon struct {
	V              string
	Cluster        cluster.Cluster
	Manifests      cluster.Manifests
	Registry       registry.Registry
	Repo           git.Repo
	Checkout       *git.Checkout
	Jobs           *job.Queue
	JobStatusCache *job.StatusCache
	EventWriter    history.EventWriter
	Logger         log.Logger
	// bookkeeping
	*LoopVars
}

// Invariant.
var _ remote.Platform = &Daemon{}

func (d *Daemon) Version() (string, error) {
	return d.V, nil
}

func (d *Daemon) Ping() error {
	return d.Cluster.Ping()
}

func (d *Daemon) Export() ([]byte, error) {
	return d.Cluster.Export()
}

func (d *Daemon) ListServices(namespace string) ([]flux.ServiceStatus, error) {
	var res []flux.ServiceStatus
	services, err := d.Cluster.AllServices(namespace)
	if err != nil {
		return nil, errors.Wrap(err, "getting services from cluster")
	}

	d.Checkout.RLock()
	defer d.Checkout.RUnlock()
	automatedServices, err := d.Manifests.ServicesWithPolicy(d.Checkout.ManifestDir(), policy.Automated)
	if err != nil {
		return nil, errors.Wrap(err, "checking service policies")
	}
	lockedServices, err := d.Manifests.ServicesWithPolicy(d.Checkout.ManifestDir(), policy.Locked)
	if err != nil {
		return nil, errors.Wrap(err, "checking service policies")
	}
	ignoredServices, err := d.Manifests.ServicesWithPolicy(d.Checkout.ManifestDir(), policy.Ignore)
	if err != nil {
		return nil, errors.Wrap(err, "checking service policies")
	}

	for _, service := range services {
		res = append(res, flux.ServiceStatus{
			ID:         service.ID,
			Containers: containers2containers(service.ContainersOrNil()),
			Status:     service.Status,
			Automated:  automatedServices.Contains(service.ID),
			Locked:     lockedServices.Contains(service.ID),
			Ignore:     ignoredServices.Contains(service.ID),
		})
	}

	return res, nil
}

// List the images available for set of services
func (d *Daemon) ListImages(spec update.ServiceSpec) ([]flux.ImageStatus, error) {
	var services []cluster.Service
	var err error
	if spec == update.ServiceSpecAll {
		services, err = d.Cluster.AllServices("")
	} else {
		id, err := spec.AsID()
		if err != nil {
			return nil, errors.Wrap(err, "treating service spec as ID")
		}
		services, err = d.Cluster.SomeServices([]flux.ServiceID{id})
	}

	images, err := update.CollectAvailableImages(d.Registry, services)
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
type DaemonJobFunc func(jobID job.ID, working *git.Checkout, logger log.Logger) (*history.CommitEventMetadata, error)

func (d *Daemon) queueJob(do DaemonJobFunc) job.ID {
	id := job.ID(guid.New())
	d.Jobs.Enqueue(&job.Job{
		ID: id,
		Do: func(logger log.Logger) error {
			started := time.Now().UTC()
			d.JobStatusCache.SetStatus(id, job.Status{StatusString: job.StatusRunning})
			// make a working clone so we don't mess with files we
			// will be reading from elsewhere
			working, err := d.Checkout.WorkingClone()
			if err != nil {
				d.JobStatusCache.SetStatus(id, job.Status{StatusString: job.StatusFailed, Err: err.Error()})
				return err
			}
			defer working.Clean()
			metadata, err := do(id, working, logger)
			if err != nil {
				d.JobStatusCache.SetStatus(id, job.Status{StatusString: job.StatusFailed, Err: err.Error()})
				return err
			}
			d.JobStatusCache.SetStatus(id, job.Status{StatusString: job.StatusSucceeded, Result: *metadata})
			logger.Log("revision", metadata.Revision)
			if metadata.Revision != "" {
				var serviceIDs []flux.ServiceID
				for id, result := range metadata.Result {
					if result.Status == update.ReleaseStatusSuccess {
						serviceIDs = append(serviceIDs, id)
					}
				}
				return d.LogEvent(history.Event{
					ServiceIDs: serviceIDs,
					Type:       history.EventCommit,
					StartedAt:  started,
					EndedAt:    started,
					LogLevel:   history.LogLevelInfo,
					Metadata:   metadata,
				})
			}
			return nil
		},
	})
	d.JobStatusCache.SetStatus(id, job.Status{StatusString: job.StatusQueued})
	return id
}

// Apply the desired changes to the config files
func (d *Daemon) UpdateManifests(spec update.Spec) (job.ID, error) {
	var id job.ID
	if spec.Type == "" {
		return id, errors.New("no type in update spec")
	}
	switch s := spec.Spec.(type) {
	case release.Changes:
		return d.queueJob(d.release(spec, s)), nil
	case policy.Updates:
		return d.queueJob(d.updatePolicy(spec, s)), nil
	default:
		return id, fmt.Errorf(`unknown update type "%s"`, spec.Type)
	}
}

func (d *Daemon) updatePolicy(spec update.Spec, updates policy.Updates) DaemonJobFunc {
	return func(jobID job.ID, working *git.Checkout, logger log.Logger) (*history.CommitEventMetadata, error) {
		// For each update
		var serviceIDs []flux.ServiceID
		metadata := &history.CommitEventMetadata{
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
			err := cluster.UpdateManifest(d.Manifests, working.ManifestDir(), string(serviceID), func(def []byte) ([]byte, error) {
				newDef, err := d.Manifests.UpdatePolicies(def, u)
				if err != nil {
					metadata.Result[serviceID] = update.ServiceResult{
						Status: update.ReleaseStatusFailed,
						Error:  err.Error(),
					}
					return nil, err
				}
				if string(newDef) == string(def) {
					metadata.Result[serviceID] = update.ServiceResult{
						Status: update.ReleaseStatusSkipped,
					}
				} else {
					serviceIDs = append(serviceIDs, serviceID)
					metadata.Result[serviceID] = update.ServiceResult{
						Status: update.ReleaseStatusSuccess,
					}
				}
				return newDef, nil
			})
			switch err {
			case cluster.ErrNoResourceFilesFoundForService, cluster.ErrMultipleResourceFilesFoundForService:
				metadata.Result[serviceID] = update.ServiceResult{
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

		if err := working.CommitAndPush(policyCommitMessage(updates, spec.Cause), &git.Note{JobID: jobID, Spec: spec}); err != nil {
			// On the chance pushing failed because it was not
			// possible to fast-forward, ask for a sync so the
			// next attempt is more likely to succeed.
			d.askForSync()
			return nil, err
		}
		if anythingAutomated {
			d.askForImagePoll()
		}

		var err error
		metadata.Revision, err = working.HeadRevision()
		if err != nil {
			return nil, err
		}
		return metadata, nil
	}
}

func (d *Daemon) release(spec update.Spec, c release.Changes) DaemonJobFunc {
	return func(jobID job.ID, working *git.Checkout, logger log.Logger) (*history.CommitEventMetadata, error) {
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
			if err := working.CommitAndPush(commitMsg, &git.Note{JobID: jobID, Spec: spec, Result: result}); err != nil {
				// On the chance pushing failed because it was not
				// possible to fast-forward, ask for a sync so the
				// next attempt is more likely to succeed.
				d.askForSync()
				return nil, err
			}
			revision, err = working.HeadRevision()
			if err != nil {
				return nil, err
			}
		}
		return &history.CommitEventMetadata{
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
func (d *Daemon) SyncNotify() error {
	d.askForSync()
	return nil
}

// Ask the daemon how far it's got committing things; in particular, is the job
// queued? running? committed? If it is done, the commit ref is returned.
func (d *Daemon) JobStatus(jobID job.ID) (job.Status, error) {
	// Is the job queued, running, or recently finished?
	status, ok := d.JobStatusCache.Status(jobID)
	if ok {
		return status, nil
	}

	// Look through the commits for a note referencing this job.  This
	// means that even if fluxd restarts, we will at least remember
	// jobs which have pushed a commit.
	if err := d.Checkout.Pull(); err != nil {
		return job.Status{}, errors.Wrap(err, "updating repo for status")
	}
	refs, err := d.Checkout.RevisionsBefore("HEAD")
	if err != nil {
		return job.Status{}, errors.Wrap(err, "checking revisions for status")
	}
	for _, ref := range refs {
		note, _ := d.Checkout.GetNote(ref)
		if note != nil && note.JobID == jobID {
			return job.Status{
				StatusString: job.StatusSucceeded,
				Result: history.CommitEventMetadata{
					Revision: ref,
					Spec:     &note.Spec,
					Result:   note.Result,
				},
			}, nil
		}
	}

	return job.Status{}, ErrUnknownJob
}

// Ask the daemon how far it's got applying things; in particular, is it
// past the supplied release? Return the list of commits between where
// we have applied and the ref given, inclusive. E.g., if you send HEAD,
// you'll get all the commits yet to be applied. If you send a hash
// and it's applied _past_ it, you'll get an empty list.
func (d *Daemon) SyncStatus(commitRef string) ([]string, error) {
	return d.Checkout.RevisionsBetween(d.Checkout.SyncTag, commitRef)
}

func (d *Daemon) GitRepoConfig(regenerate bool) (flux.GitConfig, error) {
	publicSSHKey, err := d.Cluster.PublicSSHKey(regenerate)
	if err != nil {
		return flux.GitConfig{}, err
	}
	return flux.GitConfig{
		Remote:       d.Repo.GitRemoteConfig,
		PublicSSHKey: publicSSHKey,
	}, nil
}

// Non-remote.Platform methods

func (d *Daemon) LogEvent(ev history.Event) error {
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
		id, _ := flux.ParseImageID(c.Image)
		res[i] = flux.Container{
			Name: c.Name,
			Current: flux.Image{
				ID: id,
			},
		}
	}
	return res
}

func containersWithAvailable(service cluster.Service, images update.ImageMap) (res []flux.Container) {
	for _, c := range service.ContainersOrNil() {
		id, _ := flux.ParseImageID(c.Image)
		repo := id.Repository()
		available := images[repo]
		res = append(res, flux.Container{
			Name: c.Name,
			Current: flux.Image{
				ID: id,
			},
			Available: available,
		})
	}
	return res
}

// policyEvents builds a map of events (by type), for all the events in this set of
// updates. There will be one event per type, containing all service ids
// affected by that event. e.g. all automated services will share an event.
func policyEvents(us policy.Updates, now time.Time) map[string]history.Event {
	eventsByType := map[string]history.Event{}
	for serviceID, update := range us {
		for _, eventType := range policyEventTypes(update) {
			e, ok := eventsByType[eventType]
			if !ok {
				e = history.Event{
					ServiceIDs: []flux.ServiceID{},
					Type:       eventType,
					StartedAt:  now,
					EndedAt:    now,
					LogLevel:   history.LogLevelInfo,
				}
			}
			e.ServiceIDs = append(e.ServiceIDs, serviceID)
			eventsByType[eventType] = e
		}
	}
	return eventsByType
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

// policyEventTypes is a deduped list of all event types this update contains
func policyEventTypes(u policy.Update) []string {
	types := map[string]struct{}{}
	for p, _ := range u.Add {
		switch {
		case p == policy.Automated:
			types[history.EventAutomate] = struct{}{}
		case p == policy.Locked:
			types[history.EventLock] = struct{}{}
		case policy.Tag(p):
			types[history.EventUpdateTag] = struct{}{}
		}
	}

	for p, _ := range u.Remove {
		switch {
		case p == policy.Automated:
			types[history.EventDeautomate] = struct{}{}
		case p == policy.Locked:
			types[history.EventUnlock] = struct{}{}
		case policy.Tag(p):
			types[history.EventUpdateTag] = struct{}{}
		}
	}
	var result []string
	for t := range types {
		result = append(result, t)
	}
	sort.Strings(result)
	return result
}
