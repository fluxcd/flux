package automator

import (
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/jobs"
	"github.com/weaveworks/flux/platform"
	"github.com/weaveworks/flux/release"
)

const (
	automationCycle = 60 * time.Second
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
	tick := time.Tick(automationCycle)
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

		_, err := a.cfg.Jobs.PutJob(inst.ID, automatedInstanceJob(inst.ID, time.Now()))
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

func (a *Automator) Handle(j *jobs.Job, _ jobs.JobUpdater) ([]jobs.Job, error) {
	logger := log.NewContext(a.cfg.Logger).With("job", j.ID)
	switch j.Method {
	case jobs.AutomatedInstanceJob:
		return a.handleAutomatedInstanceJob(logger, j)
	default:
		return nil, jobs.ErrUnknownJobMethod
	}
}

func (a *Automator) handleAutomatedInstanceJob(logger log.Logger, j *jobs.Job) ([]jobs.Job, error) {
	followUps := []jobs.Job{automatedInstanceJob(j.Instance, time.Now())}
	params := j.Params.(jobs.AutomatedInstanceJobParams)

	config, err := a.cfg.InstanceDB.GetConfig(params.InstanceID)
	if err != nil {
		return followUps, errors.Wrap(err, "getting instance config")
	}

	automatedServiceIDs := flux.ServiceIDSet{}
	for id, service := range config.Services {
		if service.Policy() == flux.PolicyAutomated {
			automatedServiceIDs.Add([]flux.ServiceID{id})
		}
	}

	if len(automatedServiceIDs) == 0 {
		return nil, nil
	}

	inst, err := a.cfg.Instancer.Get(params.InstanceID)
	if err != nil {
		return followUps, errors.Wrap(err, "getting job instance")
	}

	// Get all services, then filter to the automated ones.
	// It's done this way so a single missing service doesn't fail everything.
	// TODO: This should come from git not kubernetes
	allServices, err := release.AllServicesExcept(nil).SelectServices(inst)
	if err != nil {
		return followUps, errors.Wrap(err, "getting services")
	}

	// Get just the automated services we can release.
	var services []platform.Service
	for _, service := range allServices {
		if automatedServiceIDs.Contains(service.ID) {
			services = append(services, service)
		}
	}

	if len(services) == 0 {
		// No automated services are defined, don't reschedule.
		return nil, nil
	}

	// Get the images used for each automated service. We have to do this
	// ourselves, so that any individual failure doesn't error out the whole
	// job.
	images := instance.ImageMap{}
	for _, service := range services {
		for _, container := range service.ContainersOrNil() {
			repo := flux.ParseImageID(container.Image).Repository()
			images[repo] = nil
		}
	}
	for repo := range images {
		imageRepo, err := inst.GetRepository(repo)
		if err != nil {
			logger.Log("err", errors.Wrapf(err, "fetching image metadata for %s", repo))
			continue
		}
		images[repo] = imageRepo
	}

	// Calculate which services need releasing.
	updateMap := release.CalculateUpdates(services, images, func(format string, args ...interface{}) { /* noop */ })
	releases := map[flux.ImageID]flux.ServiceIDSet{}
	for serviceID, updates := range updateMap {
		for _, update := range updates {
			if releases[update.Target] == nil {
				releases[update.Target] = flux.ServiceIDSet{}
			}
			releases[update.Target].Add([]flux.ServiceID{serviceID})
		}
	}

	// Schedule the release for each image. Will be a noop if all services are
	// running latest of that image.
	for imageID, serviceIDSet := range releases {
		var serviceSpecs []flux.ServiceSpec
		for id := range serviceIDSet {
			serviceSpecs = append(serviceSpecs, flux.ServiceSpec(id))
		}
		followUps = append(followUps, jobs.Job{
			Queue: jobs.ReleaseJob,
			// Key stops us getting two jobs queued for the same service. That way if a
			// release is slow the automator won't queue a horde of jobs to upgrade it.
			Key: strings.Join([]string{
				jobs.ReleaseJob,
				string(params.InstanceID),
				string(imageID),
				"automated",
			}, "|"),
			Method:   jobs.ReleaseJob,
			Priority: jobs.PriorityBackground,
			Params: jobs.ReleaseJobParams{
				ServiceSpecs: serviceSpecs,
				ImageSpec:    flux.ImageSpec(imageID),
				Kind:         flux.ReleaseKindExecute,
			},
		})
	}

	return followUps, nil
}

func automatedInstanceJob(instanceID flux.InstanceID, now time.Time) jobs.Job {
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
		ScheduledAt: now.UTC().Add(automationCycle),
	}
}
