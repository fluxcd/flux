package automator

import (
	"time"

	"github.com/go-kit/kit/log"

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
	tick := time.Tick(automationCycle)
	for range tick {
		insts, err := a.cfg.InstanceDB.All()
		if err != nil {
			errorLogger.Log("err", err)
			continue
		}
		for _, inst := range insts {
			for service, conf := range inst.Config.Services {
				if conf.Policy() == flux.PolicyAutomated {
					a.cfg.Releaser.PutJob(inst.ID, jobs.Job{
						Method:   jobs.ReleaseJob,
						Priority: jobs.PriorityBackground,
						Params: jobs.ReleaseJobParams{
							ServiceSpec: flux.ServiceSpec(service),
							ImageSpec:   flux.ImageSpecLatest,
							Kind:        flux.ReleaseKindExecute,
						},
					})
				}
			}
		}
	}
}
