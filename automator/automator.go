package automator

import (
	"time"

	"github.com/weaveworks/fluxy"
	"github.com/weaveworks/fluxy/instance"
)

const (
	automationEnabled  = "Automation enabled."
	automationDisabled = "Automation disabled."

	HardwiredInstance = "DEFAULT"

	automationCycle = 15
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

func (a *Automator) Start() {
	tick := time.Tick(automationCycle * time.Second)
	for range tick {
		inst, err := a.cfg.InstanceDB.Get(HardwiredInstance)
		if err != nil {
			continue
		}
		for service, conf := range inst.Services {
			if conf.Automated {
				a.cfg.Releaser.PutJob(flux.ReleaseJobSpec{
					ServiceSpec: flux.ServiceSpec(service),
					ImageSpec:   flux.ImageSpecLatest,
					Kind:        flux.ReleaseKindExecute,
				})
			}
		}
	}
}

func (a *Automator) recordAutomation(service flux.ServiceID, automation bool) error {
	if err := a.cfg.InstanceDB.Update(HardwiredInstance, func(conf instance.Config) (instance.Config, error) {
		if serviceConf, found := conf.Services[service]; found {
			serviceConf.Automated = automation
			conf.Services[service] = serviceConf
		} else {
			conf.Services[service] = instance.ServiceConfig{
				Automated: true,
			}
		}
		return conf, nil
	}); err != nil {
		return err
	}
	return nil
}

// Automate turns on automated (continuous) deployment for the named service.
func (a *Automator) Automate(namespace, serviceName string) error {
	a.cfg.History.LogEvent(namespace, serviceName, automationEnabled)
	return a.recordAutomation(flux.MakeServiceID(namespace, serviceName), true)
}

// Deautomate turns off automated (continuous) deployment for the named service.
// This is more of a signal; it may take some time for the service to be
// properly deautomated.
func (a *Automator) Deautomate(namespace, serviceName string) error {
	a.cfg.History.LogEvent(namespace, serviceName, automationDisabled)
	return a.recordAutomation(flux.MakeServiceID(namespace, serviceName), false)
}

// IsAutomated checks if a given service has automation enabled.
func (a *Automator) IsAutomated(namespace, serviceName string) bool {
	if a == nil {
		return false
	}
	inst, err := a.cfg.InstanceDB.Get(HardwiredInstance)
	if err != nil {
		return false
	}

	conf, ok := inst.Services[flux.MakeServiceID(namespace, serviceName)]
	if !ok {
		return false
	}
	return conf.Automated
}
