package automator

import (
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/jobs"
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
		for service, conf := range inst.Config.Services {
			if conf.Policy() != flux.PolicyAutomated {
				continue
			}
			_, err := a.cfg.Jobs.PutJob(inst.ID, automatedServiceJob(inst.ID, service, time.Now()))
			if err != nil && err != jobs.ErrJobAlreadyQueued {
				errorLogger.Log("err", errors.Wrapf(err, "queueing automated service job"))
			}
		}
	}
}

func (a *Automator) Handle(j *jobs.Job, _ jobs.JobUpdater) ([]jobs.Job, error) {
	params := j.Params.(jobs.AutomatedServiceJobParams)

	serviceID, err := flux.ParseServiceID(string(params.ServiceSpec))
	if err != nil {
		// I don't see how we could ever expect this to work, so let's not
		// reschedule.
		return nil, errors.Wrapf(err, "parsing service ID from spec %s", params.ServiceSpec)
	}

	followUps := []jobs.Job{
		automatedServiceJob(j.Instance, flux.ServiceID(params.ServiceSpec), time.Now()),
	}

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
		return followUps, nil
	}

	followUps = append(followUps, jobs.Job{
		Queue: jobs.ReleaseJob,
		// Key stops us getting two jobs queued for the same service. That way if a
		// release is slow the automator won't queue a horde of jobs to upgrade it.
		Key: strings.Join([]string{
			jobs.ReleaseJob,
			string(j.Instance),
			string(params.ServiceSpec),
			string(flux.ImageSpecLatest),
			"automated",
		}, "|"),
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

func automatedServiceJob(instanceID flux.InstanceID, serviceID flux.ServiceID, now time.Time) jobs.Job {
	return jobs.Job{
		Queue: jobs.AutomatedServiceJob,
		// Key stops us getting two jobs for the same service
		Key: strings.Join([]string{
			jobs.AutomatedServiceJob,
			string(instanceID),
			string(serviceID),
		}, "|"),
		Method:   jobs.AutomatedServiceJob,
		Priority: jobs.PriorityBackground,
		Params: jobs.AutomatedServiceJobParams{
			ServiceSpec: flux.ServiceSpec(serviceID),
		},
		ScheduledAt: now.UTC().Add(automationCycle),
	}
}
