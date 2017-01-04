package automator

import (
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/jobs"
	"github.com/weaveworks/flux/platform/kubernetes"
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

		_, err := a.cfg.Jobs.PutJob(inst.ID, jobs.Job{
			// Key stops us getting two jobs for the same service
			Key: strings.Join([]string{
				jobs.AutomatedInstanceJob,
				string(inst.ID),
			}, "|"),
			Method:   jobs.AutomatedInstanceJob,
			Priority: jobs.PriorityBackground,
			Params: jobs.AutomatedInstanceJobParams{
				InstanceID: inst.ID,
			},
		})
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
	case jobs.AutomatedServiceJob:
		return a.handleAutomatedServiceJob(logger, j)
	case jobs.AutomatedInstanceJob:
		return a.handleAutomatedInstanceJob(logger, j)
	default:
		return nil, jobs.ErrUnknownJobMethod
	}
}

func (a *Automator) handleAutomatedServiceJob(logger log.Logger, j *jobs.Job) ([]jobs.Job, error) {
	params := j.Params.(jobs.AutomatedServiceJobParams)
	serviceID, err := flux.ParseServiceID(string(params.ServiceSpec))
	if err != nil {
		// I don't see how we could ever expect this to work, so let's not
		// reschedule.
		return nil, errors.Wrapf(err, "parsing service ID from spec %s", params.ServiceSpec)
	}

	j.ScheduledAt = j.ScheduledAt.Add(automationCycle)
	followUps := []jobs.Job{*j}

	config, err := a.cfg.InstanceDB.GetConfig(j.Instance)
	if err != nil {
		return followUps, errors.Wrap(err, "getting instance config")
	}

	s := config.Services[serviceID]
	if !s.Automated {
		// Job is not automated, don't reschedule
		return nil, nil
	}
	if s.Locked {
		// Just locked, might work at some point.
		return followUps, nil
	}

	followUps = append(followUps, jobs.Job{
		Method:   jobs.ReleaseJob,
		Priority: jobs.PriorityBackground,
		Params: jobs.ReleaseJobParams{
			ServiceSpec: params.ServiceSpec,
			ImageSpec:   flux.ImageSpecLatest,
			Kind:        flux.ReleaseKindExecute,
		},
	})
	return followUps, nil
}

func (a *Automator) handleAutomatedInstanceJob(logger log.Logger, j *jobs.Job) ([]jobs.Job, error) {
	j.ScheduledAt = j.ScheduledAt.Add(automationCycle)
	followUps := []jobs.Job{*j}

	params := j.Params.(jobs.AutomatedInstanceJobParams)
	config, err := a.cfg.InstanceDB.GetConfig(params.InstanceID)
	if err != nil {
		return followUps, errors.Wrap(err, "getting instance config")
	}

	if !a.hasAutomatedServices(config.Services) {
		// All services have been deautomated. Don't reschedule.
		return nil, nil
	}

	inst, err := a.cfg.Instancer.Get(params.InstanceID)
	if err != nil {
		return followUps, errors.Wrap(err, "getting job instance")
	}

	// Clone the repo
	path, _, err := inst.ConfigRepo().Clone()
	if err != nil {
		return followUps, errors.Wrap(err, "cloning config repo")
	}

	// Get all defined services
	// TODO: This should handle multi-document files
	serviceFiles, err := kubernetes.DefinedServices(path)
	if err != nil {
		return followUps, errors.Wrap(err, "finding defined services")
	}

	// Get the intersection of defined and automated services
	var serviceIDs []flux.ServiceID
	for id, service := range config.Services {
		if service.Policy() != flux.PolicyAutomated {
			continue
		}
		if _, ok := serviceFiles[id]; !ok {
			// Service is automated, but undefined. Skip it.
			// TODO: Log something here, probably.
			continue
		}
		serviceIDs = append(serviceIDs, id)
	}

	if len(serviceIDs) == 0 {
		// No automated services are defined, don't reschedule.
		return nil, fmt.Errorf("no definitions found for automated services")
	}

	// Get the images used for each automated service
	var repos []string
	for _, serviceID := range serviceIDs {
		// TODO: Pass in the platform, don't just assume kubernetes here
		for _, path := range serviceFiles[serviceID] {
			definition, err := ioutil.ReadFile(path) // TODO: not multi-doc safe
			if err != nil {
				return nil, errors.Wrapf(err, "reading definition file for %s: %s", serviceID, path)
			}
			images, err := kubernetes.ImagesForDefinition(definition)
			if err != nil {
				return nil, errors.Wrapf(err, "reading definition file for %s: %s", serviceID, path)
			}
			for _, image := range images {
				repos = append(repos, image.Repository())
			}
		}
	}

	images, err := inst.CollectAvailableImages(repos)
	if err != nil {
		return followUps, errors.Wrap(err, "collecting available images")
	}

	serviceSpecs := make([]flux.ServiceSpec, len(serviceIDs))
	for _, id := range serviceIDs {
		serviceSpecs = append(serviceSpecs, flux.ServiceSpec(id))
	}

	// Schedule the release for each image. Will be a noop if all services are
	// running latest of that image.
	for image := range images {
		latest := images.LatestImage(image)
		if latest == nil {
			continue
		}
		followUps = append(followUps, jobs.Job{
			Method:   jobs.ReleaseJob,
			Priority: jobs.PriorityBackground,
			Params: jobs.ReleaseJobParams{
				ServiceSpecs: serviceSpecs,
				ImageSpec:    flux.ImageSpec(latest.ID),
				Kind:         flux.ReleaseKindExecute,
			},
		})
	}

	return followUps, nil
}
