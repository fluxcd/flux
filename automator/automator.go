package automator

import (
	"sync"

	"github.com/weaveworks/fluxy"
	"github.com/weaveworks/fluxy/instance"
)

const (
	automationEnabled  = "Automation enabled."
	automationDisabled = "Automation disabled."

	HardwiredInstance = "DEFAULT"
)

// Automator orchestrates continuous deployment for specific services.
type Automator struct {
	cfg    Config
	mtx    sync.RWMutex
	active map[flux.ServiceID]*svc
}

// New creates a new automator.
func New(cfg Config) (*Automator, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	auto := &Automator{
		cfg:    cfg,
		active: map[flux.ServiceID]*svc{},
	}
	if err := auto.recoverAutomation(); err != nil {
		return nil, err
	}
	return auto, nil
}

func (a *Automator) recordAutomation(service flux.ServiceID, automation bool) error {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	if err := a.cfg.InstanceDB.Update(HardwiredInstance, func(conf instance.InstanceConfig) (instance.InstanceConfig, error) {
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

func (a *Automator) recoverAutomation() error {
	conf, err := a.cfg.InstanceDB.Get(HardwiredInstance)
	if err != nil {
		return err
	}
	for service, serviceConf := range conf.Services {
		if serviceConf.Automated {
			if err = a.automate(service); err != nil {
				return err
			}
		}
	}
	return nil
}

// Automate turns on automated (continuous) deployment for the named service.
// This call always succeeds; if the named service cannot be automated for some
// reason, that will be detected and happen autonomously.
func (a *Automator) Automate(namespace, serviceName string) error {
	a.cfg.History.LogEvent(namespace, serviceName, automationEnabled)
	service := flux.MakeServiceID(namespace, serviceName)
	if err := a.recordAutomation(service, true); err != nil {
		return err
	}
	return a.automate(service)
}

// Deautomate turns off automated (continuous) deployment for the named service.
// This is more of a signal; it may take some time for the service to be
// properly deautomated.
func (a *Automator) Deautomate(namespace, serviceName string) error {
	a.cfg.History.LogEvent(namespace, serviceName, automationDisabled)
	service := flux.MakeServiceID(namespace, serviceName)
	if err := a.recordAutomation(service, false); err != nil {
		return err
	}
	return a.deautomate(service)
}

func (a *Automator) automate(service flux.ServiceID) error {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	if _, ok := a.active[service]; ok {
		return nil
	}

	onDelete := func() { a.deleteCallback(service) }
	svcLogFunc := makeServiceLogFunc(a.cfg.History, service)
	s := newSvc(service, svcLogFunc, onDelete, a.cfg)
	a.active[service] = s
	return nil
}

// Deautomate turns off automated (continuous) deployment for the named service.
// This is more of a signal; it may take some time for the service to be
// properly deautomated.
func (a *Automator) deautomate(service flux.ServiceID) error {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	s, ok := a.active[service]
	if !ok {
		return nil
	}

	// We signal delete rather than actually deleting anything here,
	// to make sure svc termination follows a single code path.
	s.signalDelete()
	return nil
}

// IsAutomated checks if a given service has automation enabled.
func (a *Automator) IsAutomated(namespace, serviceName string) bool {
	if a == nil {
		return false
	}
	a.mtx.RLock()
	_, ok := a.active[flux.MakeServiceID(namespace, serviceName)]
	a.mtx.RUnlock()
	return ok
}

// deleteCallback is invoked by a svc when it shuts down. A svc may terminate
// itself, and so needs this as a form of accounting.
func (a *Automator) deleteCallback(service flux.ServiceID) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	delete(a.active, service)
}
