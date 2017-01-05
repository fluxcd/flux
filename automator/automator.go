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
			_, err := a.cfg.Jobs.PutJob(inst.ID, jobs.Job{
				// Key stops us getting two jobs for the same service
				Key: strings.Join([]string{
					jobs.AutomatedServiceJob,
					string(inst.ID),
					string(service),
				}, "|"),
				Method:   jobs.AutomatedServiceJob,
				Priority: jobs.PriorityBackground,
				Params: jobs.AutomatedServiceJobParams{
					ServiceSpec: flux.ServiceSpec(service),
				},
			})
			if err != nil && err != jobs.ErrJobAlreadyQueued {
				errorLogger.Log("err", errors.Wrapf(err, "queueing automated service job"))
			}
		}
	}
}

func (a *Automator) Handle(j *jobs.Job, _ jobs.JobUpdater) error {
	logger := log.NewContext(a.cfg.Logger).With("job", j.ID)
	params := j.Params.(jobs.AutomatedServiceJobParams)

	serviceID, err := flux.ParseServiceID(string(params.ServiceSpec))
	if err != nil {
		// I don't see how we could ever expect this to work, so let's not
		// reschedule.
		return errors.Wrapf(err, "parsing service ID from spec %s", params.ServiceSpec)
	}

	config, err := a.cfg.InstanceDB.GetConfig(j.Instance)
	if err != nil {
		if err2 := a.reschedule(j); err2 != nil {
			logger.Log("err", err2) // abnormal
		}
		return errors.Wrap(err, "getting instance config")
	}

	s := config.Services[serviceID]
	if !s.Automated {
		// Job is not automated, don't reschedule
		return nil
	}
	if s.Locked {
		return a.reschedule(j)
	}

	if _, err := a.cfg.Jobs.PutJob(j.Instance, jobs.Job{
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
	}); err != nil && err != jobs.ErrJobAlreadyQueued {
		logger.Log("err", errors.Wrap(err, "put automated release job")) // abnormal
	}

	return a.reschedule(j)
}

func (a *Automator) reschedule(j *jobs.Job) error {
	j.ScheduledAt = j.ScheduledAt.Add(automationCycle)
	if _, err := a.cfg.Jobs.PutJobIgnoringDuplicates(j.Instance, *j); err != nil {
		return errors.Wrap(err, "rescheduling check automated service job") // abnormal
	}
	return nil
}
