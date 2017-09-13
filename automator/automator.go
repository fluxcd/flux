package automator

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/jobs"
	"github.com/weaveworks/flux/release"
)

// Automator orchestrates continuous deployment for specific services.
type Automator struct {
	cfg Config
}

// New creates a new automator.
func New(cfg Config) (*Automator, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Automator{
		cfg: cfg,
	}, nil
}

func (a *Automator) Start(errorLogger log.Logger) {
	a.checkAll(errorLogger)
	tick := time.Tick(a.cfg.Period)
	for range tick {
		a.checkAll(errorLogger)
	}
}

func (a *Automator) checkAll(errorLogger log.Logger) {
	insts, err := a.cfg.InstanceDB.All()
	if err != nil {
		errorLogger.Log("err", err)
		return
	}
	for _, inst := range insts {
		if !a.hasAutomatedServices(inst.Config.Services) {
			continue
		}

		_, err := a.cfg.Jobs.PutJob(inst.ID, a.automatedInstanceJob(inst.ID, time.Now()))
		if err != nil && err != jobs.ErrJobAlreadyQueued {
			errorLogger.Log("err", errors.Wrapf(err, "queueing automated instance job"))
		}
	}
}

func (a *Automator) hasAutomatedServices(services map[flux.ServiceID]instance.ServiceConfig) bool {
	for _, service := range services {
		if service.Policy() == flux.PolicyAutomated {
			return true
		}
	}
	return false
}

func (a *Automator) Handle(j *jobs.Job, updater jobs.JobUpdater) ([]jobs.Job, error) {
	logger := log.NewContext(a.cfg.Logger).With("job", j.ID)
	switch j.Method {
	case jobs.AutomatedInstanceJob:
		return a.handleAutomatedInstanceJob(logger, j, updater)
	default:
		return nil, jobs.ErrUnknownJobMethod
	}
}

func (a *Automator) handleAutomatedInstanceJob(logger log.Logger, job *jobs.Job, updater jobs.JobUpdater) ([]jobs.Job, error) {
	followUps := []jobs.Job{a.automatedInstanceJob(job.Instance, time.Now())}
	params := job.Params.(jobs.AutomatedInstanceJobParams)

	config, err := a.cfg.InstanceDB.GetConfig(params.InstanceID)
	if err != nil {
		return followUps, errors.Wrap(err, "getting instance config")
	}

	automatedServiceIDs := []flux.ServiceID{}
	for id, service := range config.Services {
		if service.Policy() == flux.PolicyAutomated {
			automatedServiceIDs = append(automatedServiceIDs, id)
		}
	}

	if len(automatedServiceIDs) == 0 {
		return nil, nil
	}

	inst, err := a.cfg.Instancer.Get(params.InstanceID)
	if err != nil {
		return followUps, errors.Wrap(err, "getting job instance")
	}

	rc := release.NewReleaseContext(inst)
	if err = rc.CloneRepo(); err != nil {
		return followUps, errors.Wrap(err, "cloning repo")
	}
	defer rc.Clean()

	results := flux.ReleaseResult{}

	logInJob := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		job.Log = append(job.Log, msg)
		updater.UpdateJob(*job)
	}

	// Get the list of services that are automated, in the repo, and in the running service.
	updates, err := rc.SelectServices(
		results,
		logInJob,
		&release.IncludeFilter{
			IDs: automatedServiceIDs,
		},
	)
	if err != nil {
		logInJob("error finding services: %s", err)
		return followUps, err
	}

	// No services that are automated exist. Don't check again.
	if len(updates) == 0 {
		logInJob("no services to update; descheduling automatic check")
		return nil, fmt.Errorf("no automated service(s) %s exist in config or running system", automatedServiceIDs)
	}

	// Get the images available for each automated service.
	images, err := release.CollectAvailableImages(rc.Instance, updates)
	if err != nil {
		logInJob("error fetching image updates: %s", err)
		return followUps, errors.Wrap(err, "fetching image updates")
	}

	// At this point we have all the data we need to know precisely
	// what needs updating. However, we want to break this down into
	// individual jobs that can be scheduled, rather than doing it all
	// inline here, since that is closer to the ideal of reacting to
	// each new image appearing (as well as being more incremental and
	// thus less risky).

	// We effectively need a transpose of what we have so far. To get
	// there in one pass, we look through the _services_, since we
	// already have a map of the available images.
	imageServices := map[flux.ImageID][]flux.ServiceSpec{}
	for _, update := range updates {
		for _, container := range update.Service.ContainersOrNil() {
			currentImageID, err := flux.ParseImageID(container.Image)
			if err != nil {
				logInJob("error parsing image in service %s container %s (%q): %s", update.Service.ID, container.Name, container.Image, err)
				return followUps, errors.Wrapf(err, "calculating image updates for %s", container.Name)
			}
			if latest := images.LatestImage(currentImageID.Repository()); latest != nil && latest.ID != currentImageID {
				imageServices[latest.ID] = append(imageServices[latest.ID], flux.ServiceSpec(update.ServiceID))
			}
		}
	}

	for imageID, services := range imageServices {
		logInJob("scheduling release of image %s to services %s", imageID, services)
		followUps = append(followUps, jobs.Job{
			Queue: jobs.ReleaseJob,
			// Key stops us getting two jobs queued for the same
			// service. That way if a release is slow the automator
			// won't queue a horde of jobs to upgrade it.
			Key: strings.Join([]string{
				jobs.ReleaseJob,
				string(params.InstanceID),
				imageID.String(),
				"automated",
			}, "|"),
			Method:   jobs.ReleaseJob,
			Priority: jobs.PriorityBackground,
			Params: jobs.ReleaseJobParams{
				ReleaseSpec: flux.ReleaseSpec{
					ServiceSpecs: services,
					ImageSpec:    flux.ImageSpecFromID(imageID),
					Kind:         flux.ReleaseKindExecute,
				},
				Cause: flux.ReleaseCause{
					User:    flux.UserAutomated,
					Message: fmt.Sprintf("due to new image %s", imageID.String()),
				},
			},
		})
	}
	return followUps, nil
}

func (a *Automator) automatedInstanceJob(instanceID flux.InstanceID, now time.Time) jobs.Job {
	return jobs.Job{
		Queue: jobs.AutomatedInstanceJob,
		// Key stops us getting two jobs for the same instance
		Key: strings.Join([]string{
			jobs.AutomatedInstanceJob,
			string(instanceID),
		}, "|"),
		Method:   jobs.AutomatedInstanceJob,
		Priority: jobs.PriorityBackground,
		Params: jobs.AutomatedInstanceJobParams{
			InstanceID: instanceID,
		},
		ScheduledAt: now.UTC().Add(a.cfg.Period),
	}
}
