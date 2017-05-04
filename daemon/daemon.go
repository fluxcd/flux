package daemon

import (
	"bytes"
	"fmt"
	"sort"
	gosync "sync"
	"time"

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

var ErrUnknownJob = fmt.Errorf("unkown job")

// Combine these things to form Devasta^Wan implementation of
// Platform.
type Daemon struct {
	V              string
	Cluster        cluster.Cluster
	Registry       registry.Registry
	Repo           git.Repo
	Checkout       git.Checkout
	Jobs           *job.Queue
	JobStatusCache *job.StatusCache
	EventWriter    history.EventWriter
	// bookkeeping
	syncSoon     chan struct{}
	initSyncSoon gosync.Once
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

	automatedServices, err := d.Cluster.ServicesWithPolicy(d.Checkout.ManifestDir(), policy.Automated)
	if err != nil {
		return nil, errors.Wrap(err, "checking service policies")
	}
	lockedServices, err := d.Cluster.ServicesWithPolicy(d.Checkout.ManifestDir(), policy.Locked)
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

	images, err := release.CollectAvailableImages(d.Registry, services)
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

type JobFunc func(jobID job.ID, working git.Checkout) (interface{}, error)

func (d *Daemon) queueJob(do JobFunc) job.ID {
	// TODO record job as current upon Do'ing (or in the loop)
	id := job.ID(guid.New())
	d.Jobs.Enqueue(&job.Job{
		ID: id,
		// make a working clone so we don't mess with files we will be
		// reading from elsewhere
		Do: func() error {
			d.JobStatusCache.SetStatus(id, job.Status{StatusString: job.StatusRunning})
			working, err := d.Checkout.WorkingClone()
			if err != nil {
				d.JobStatusCache.SetStatus(id, job.Status{StatusString: job.StatusFailed, Error: err})
				return err
			}
			defer working.Clean()
			result, err := do(id, working)
			if err != nil {
				d.JobStatusCache.SetStatus(id, job.Status{StatusString: job.StatusFailed, Error: err})
				return err
			}
			d.JobStatusCache.SetStatus(id, job.Status{StatusString: job.StatusSucceeded, Result: result})
			return nil
		},
	})
	d.JobStatusCache.SetStatus(id, job.Status{StatusString: job.StatusQueued})
	println("enqueued job " + id)
	return id
}

// Apply the desired changes to the config files
func (d *Daemon) UpdateManifests(spec update.Spec) (job.ID, error) {
	switch s := spec.Spec.(type) {
	case update.ReleaseSpec:
		return d.queueJob(func(jobID job.ID, working git.Checkout) (interface{}, error) {
			rc := release.NewReleaseContext(d.Cluster, d.Registry, working)
			revision, result, err := release.Release(rc, s)
			return history.CommitEventMetadata{
				Revision: revision,
				Spec:     spec,
				Result:   result,
			}, err
		}), nil
	case policy.Updates:
		return d.queueJob(func(jobID job.ID, working git.Checkout) (interface{}, error) {
			started := time.Now().UTC()
			// For each update
			var serviceIDs []flux.ServiceID
			for serviceID, update := range s {
				serviceIDs = append(serviceIDs, serviceID)
				// find the service manifest
				err := d.Cluster.UpdateManifest(working.ManifestDir(), string(serviceID), func(def []byte) ([]byte, error) {
					return d.Cluster.UpdatePolicies(def, update)
				})
				if err != nil {
					return nil, err
				}
			}

			if err := working.CommitAndPush(policyCommitMessage(s, started), &git.Note{JobID: jobID, Spec: spec}); err != nil {
				return nil, err
			}

			revision, err := working.HeadRevision()
			if err != nil {
				return nil, err
			}
			metadata := history.CommitEventMetadata{
				Revision: revision,
				Spec:     spec,
				// FIXME: include the service results here, so they get printed.
			}
			return metadata, d.LogEvent(history.Event{
				ServiceIDs: serviceIDs,
				Type:       history.EventCommit,
				StartedAt:  started,
				EndedAt:    started,
				LogLevel:   history.LogLevelInfo,
				Metadata:   metadata,
			})
		}), nil
	default:
		var id job.ID
		return id, fmt.Errorf(`unknown update type "%s"`, spec.Type)
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

	// is there a commit for this job?
	// Look through the commits for a note referencing this job. What a hack.
	// But, it means that even if fluxd restarts, we will remember jobs which
	// have pushed a commit.
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
			return job.Status{StatusString: job.StatusSucceeded, Result: ref}, nil
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
	if err := d.Checkout.Pull(); err != nil {
		return nil, errors.Wrap(err, "updating repo for status")
	}
	rc := release.NewReleaseContext(d.Cluster, d.Registry, d.Checkout)
	return rc.ListRevisions(commitRef)
}

// Non-remote.Platform methods

// `logEvent` expects the result of applying updates, and records an event in
// the history about the release taking place. It returns the origin error if
// that was non-nil, otherwise the result of the attempted logging.
func (d *Daemon) logRelease(executeErr error, release update.Release) error {
	errorMessage := ""
	logLevel := history.LogLevelInfo
	if executeErr != nil {
		errorMessage = executeErr.Error()
		logLevel = history.LogLevelError
	}

	var serviceIDs []flux.ServiceID
	for _, id := range release.Result.ServiceIDs() {
		serviceIDs = append(serviceIDs, flux.ServiceID(id))
	}

	err := d.LogEvent(history.Event{
		ServiceIDs: serviceIDs,
		Type:       history.EventRelease,
		StartedAt:  release.StartedAt,
		EndedAt:    release.EndedAt,
		LogLevel:   logLevel,
		Metadata: history.ReleaseEventMetadata{
			Release: release,
			Error:   errorMessage,
		},
	})
	if err != nil {
		if executeErr == nil {
			return errors.Wrap(err, "logging event")
		}
	}
	return executeErr
}

func (d *Daemon) LogEvent(ev history.Event) error {
	if d.EventWriter == nil {
		return nil
	}
	return d.EventWriter.LogEvent(ev)
}

// vvv helpers vvv

func containers2containers(cs []cluster.Container) []flux.Container {
	res := make([]flux.Container, len(cs))
	for i, c := range cs {
		id, _ := flux.ParseImageID(c.Image)
		res[i] = flux.Container{
			Name: c.Name,
			Current: flux.ImageDescription{
				ID: id,
			},
		}
	}
	return res
}

func containersWithAvailable(service cluster.Service, images release.ImageMap) (res []flux.Container) {
	for _, c := range service.ContainersOrNil() {
		id, _ := flux.ParseImageID(c.Image)
		repo := id.Repository()
		available := images[repo]
		res = append(res, flux.Container{
			Name: c.Name,
			Current: flux.ImageDescription{
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

func policyCommitMessage(us policy.Updates, now time.Time) string {
	events := policyEvents(us, now)
	commitMsg := &bytes.Buffer{}
	prefix := ""
	if len(events) > 1 {
		fmt.Fprintf(commitMsg, "Updated service policies:\n\n")
		prefix = "- "
	}
	for _, event := range events {
		fmt.Fprintf(commitMsg, "%s%v\n", prefix, event)
	}
	return commitMsg.String()
}

// policyEventTypes is a deduped list of all event types this update contains
func policyEventTypes(u policy.Update) []string {
	types := map[string]struct{}{}
	for _, p := range u.Add {
		switch p {
		case policy.Automated:
			types[history.EventAutomate] = struct{}{}
		case policy.Locked:
			types[history.EventLock] = struct{}{}
		}
	}

	for _, p := range u.Remove {
		switch p {
		case policy.Automated:
			types[history.EventDeautomate] = struct{}{}
		case policy.Locked:
			types[history.EventUnlock] = struct{}{}
		}
	}
	var result []string
	for t := range types {
		result = append(result, t)
	}
	sort.Strings(result)
	return result
}
