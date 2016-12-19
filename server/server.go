package server

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/history"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/jobs"
	fluxmetrics "github.com/weaveworks/flux/metrics"
	"github.com/weaveworks/flux/platform"
	"github.com/weaveworks/flux/registry"
)

const (
	serviceAutomated   = "Automation enabled."
	serviceDeautomated = "Automation disabled."

	serviceLocked   = "Service locked."
	serviceUnlocked = "Service unlocked."
)

type Server struct {
	instancer   instance.Instancer
	config      instance.DB
	messageBus  platform.MessageBus
	jobs        jobs.JobStore
	logger      log.Logger
	maxPlatform chan struct{} // semaphore for concurrent calls to the platform
	metrics     Metrics
	connected   int32
}

type Metrics struct {
	StatusDuration         metrics.Histogram
	ListServicesDuration   metrics.Histogram
	ListImagesDuration     metrics.Histogram
	HistoryDuration        metrics.Histogram
	RegisterDaemonDuration metrics.Histogram
	ConnectedDaemons       metrics.Gauge
	PlatformMetrics        platform.Metrics
}

func New(
	instancer instance.Instancer,
	config instance.DB,
	messageBus platform.MessageBus,
	jobs jobs.JobStore,
	logger log.Logger,
	metrics Metrics,
) *Server {
	metrics.ConnectedDaemons.Set(0)
	return &Server{
		instancer:   instancer,
		config:      config,
		messageBus:  messageBus,
		jobs:        jobs,
		logger:      logger,
		maxPlatform: make(chan struct{}, 8),
		metrics:     metrics,
	}
}

// The server methods are deliberately awkward, cobbled together from existing
// platform and registry APIs. I want to avoid changing those components until I
// get something working. There's also a lot of code duplication here for the
// same reason: let's not add abstraction until it's merged, or nearly so, and
// it's clear where the abstraction should exist.

func (s *Server) Status(inst flux.InstanceID) (res flux.Status, err error) {
	defer func(begin time.Time) {
		s.metrics.StatusDuration.With(
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	helper, err := s.instancer.Get(inst)
	if err != nil {
		return res, errors.Wrapf(err, "getting instance")
	}

	config, err := helper.GetConfig()
	if err != nil {
		return res, errors.Wrapf(err, "getting config for %s", inst)
	}
	// TODO: This should really check we have access permissions.
	res.Git.Configured = config.Settings.Git.URL != "" && config.Settings.Git.Key != ""

	res.Fluxd.Connected = (helper.Ping() == nil)

	return res, nil
}

func (s *Server) ListServices(inst flux.InstanceID, namespace string) (res []flux.ServiceStatus, err error) {
	defer func(begin time.Time) {
		s.metrics.ListServicesDuration.With(
			fluxmetrics.LabelNamespace, namespace,
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	helper, err := s.instancer.Get(inst)
	if err != nil {
		return nil, errors.Wrapf(err, "getting instance")
	}

	services, err := helper.GetAllServices(namespace)
	if err != nil {
		return nil, errors.Wrap(err, "getting services from platform")
	}

	config, err := helper.GetConfig()
	if err != nil {
		return nil, errors.Wrapf(err, "getting config for %s", inst)
	}

	for _, service := range services {
		if _, err := service.ContainersOrError(); err != nil {
			helper.Log("service", service.ID, "err", err)
		}
		res = append(res, flux.ServiceStatus{
			ID:         service.ID,
			Containers: containers2containers(service.ContainersOrNil()),
			Status:     service.Status,
			Automated:  config.Services[service.ID].Automated,
			Locked:     config.Services[service.ID].Locked,
		})
	}
	return res, nil
}

func containers2containers(cs []platform.Container) []flux.Container {
	res := make([]flux.Container, len(cs))
	for i, c := range cs {
		res[i] = flux.Container{
			Name: c.Name,
			Current: flux.ImageDescription{
				ID: flux.ParseImageID(c.Image),
			},
		}
	}
	return res
}

func (s *Server) ListImages(inst flux.InstanceID, spec flux.ServiceSpec) (res []flux.ImageStatus, err error) {
	defer func(begin time.Time) {
		s.metrics.ListImagesDuration.With(
			"service_spec", fmt.Sprint(spec),
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	helper, err := s.instancer.Get(inst)
	if err != nil {
		return nil, errors.Wrapf(err, "getting instance")
	}

	var services []platform.Service
	if spec == flux.ServiceSpecAll {
		services, err = helper.GetAllServices("")
	} else {
		id, err := spec.AsID()
		if err != nil {
			return nil, errors.Wrap(err, "treating service spec as ID")
		}
		services, err = helper.GetServices([]flux.ServiceID{id})
	}

	images, err := helper.CollectAvailableImages(services)
	if err != nil {
		return nil, errors.Wrap(err, "getting images for services")
	}

	for _, service := range services {
		containers := containersWithAvailable(service, images)
		res = append(res, flux.ImageStatus{
			ID:         service.ID,
			Containers: containers,
		})
	}

	return res, nil
}

func containersWithAvailable(service platform.Service, images instance.ImageMap) (res []flux.Container) {
	for _, c := range service.ContainersOrNil() {
		id := flux.ParseImageID(c.Image)
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

func (s *Server) History(inst flux.InstanceID, spec flux.ServiceSpec) (res []flux.HistoryEntry, err error) {
	defer func(begin time.Time) {
		s.metrics.HistoryDuration.With(
			"service_spec", fmt.Sprint(spec),
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	helper, err := s.instancer.Get(inst)
	if err != nil {
		return nil, errors.Wrapf(err, "getting instance")
	}

	var events []history.Event
	if spec == flux.ServiceSpecAll {
		events, err = helper.AllEvents()
		if err != nil {
			return nil, errors.Wrap(err, "fetching all history events")
		}
	} else {
		id, err := flux.ParseServiceID(string(spec))
		if err != nil {
			return nil, errors.Wrapf(err, "parsing service ID from spec %s", spec)
		}

		namespace, service := id.Components()
		events, err = helper.EventsForService(namespace, service)
		if err != nil {
			return nil, errors.Wrapf(err, "fetching history events for %s", id)
		}
	}

	res = make([]flux.HistoryEntry, len(events))
	for i, event := range events {
		res[i] = flux.HistoryEntry{
			Stamp: &event.Stamp,
			Type:  "v0",
			Data:  fmt.Sprintf("%s: %s", event.Service, event.Msg),
		}
	}

	return res, nil
}

func (s *Server) Automate(instID flux.InstanceID, service flux.ServiceID) error {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return err
	}
	ns, svc := service.Components()
	inst.LogEvent(ns, svc, serviceAutomated)
	return recordAutomated(inst, service, true)
}

func (s *Server) Deautomate(instID flux.InstanceID, service flux.ServiceID) error {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return err
	}
	ns, svc := service.Components()
	inst.LogEvent(ns, svc, serviceDeautomated)
	return recordAutomated(inst, service, false)
}

func recordAutomated(inst *instance.Instance, service flux.ServiceID, automated bool) error {
	return inst.UpdateConfig(func(conf instance.Config) (instance.Config, error) {
		if serviceConf, found := conf.Services[service]; found {
			serviceConf.Automated = automated
			conf.Services[service] = serviceConf
		} else if automated {
			conf.Services[service] = instance.ServiceConfig{
				Automated: true,
			}
		}
		return conf, nil
	})
}

func (s *Server) Lock(instID flux.InstanceID, service flux.ServiceID) error {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return err
	}
	ns, svc := service.Components()
	inst.LogEvent(ns, svc, serviceLocked)
	return recordLock(inst, service, true)
}

func (s *Server) Unlock(instID flux.InstanceID, service flux.ServiceID) error {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return err
	}
	ns, svc := service.Components()
	inst.LogEvent(ns, svc, serviceUnlocked)
	return recordLock(inst, service, false)
}

func recordLock(inst *instance.Instance, service flux.ServiceID, locked bool) error {
	if err := inst.UpdateConfig(func(conf instance.Config) (instance.Config, error) {
		if serviceConf, found := conf.Services[service]; found {
			serviceConf.Locked = locked
			conf.Services[service] = serviceConf
		} else if locked {
			conf.Services[service] = instance.ServiceConfig{
				Locked: true,
			}
		}
		return conf, nil
	}); err != nil {
		return err
	}
	return nil
}

func (s *Server) PostRelease(inst flux.InstanceID, params jobs.ReleaseJobParams) (jobs.JobID, error) {
	return s.jobs.PutJob(inst, jobs.Job{
		Queue:    jobs.ReleaseJob,
		Method:   jobs.ReleaseJob,
		Priority: jobs.PriorityInteractive,
		Params:   params,
	})
}

func (s *Server) GetRelease(inst flux.InstanceID, id jobs.JobID) (jobs.Job, error) {
	j, err := s.jobs.GetJob(inst, id)
	if err != nil {
		return jobs.Job{}, err
	}
	if j.Method != jobs.ReleaseJob {
		return jobs.Job{}, fmt.Errorf("job is not a release")
	}
	return j, err
}

func (s *Server) GetConfig(instID flux.InstanceID) (flux.InstanceConfig, error) {
	fullConfig, err := s.config.GetConfig(instID)
	if err != nil {
		return flux.InstanceConfig{}, nil
	}

	config := flux.InstanceConfig(fullConfig.Settings)
	return config, nil
}

func (s *Server) SetConfig(instID flux.InstanceID, updates flux.UnsafeInstanceConfig) error {
	if _, err := registry.CredentialsFromConfig(updates); err != nil {
		return errors.Wrap(err, "invalid registry credentials")
	}
	return s.config.UpdateConfig(instID, applyConfigUpdates(updates))
}

func applyConfigUpdates(updates flux.UnsafeInstanceConfig) instance.UpdateFunc {
	return func(config instance.Config) (instance.Config, error) {
		config.Settings = updates
		return config, nil
	}
}

// RegisterDaemon handles a daemon connection. It blocks until the
// daemon is disconnected.
//
// There are two conditions where we need to close and cleanup: either
// the server has initiated a close (due to another client showing up,
// say) or the client has disconnected.
//
// If the server has initiated a close, we should close the other
// client's respective blocking goroutine.
//
// If the client has disconnected, there is no way to detect this in
// go, aside from just trying to connection. Therefore, the server
// will get an error when we try to use the client. We rely on that to
// break us out of this method.
func (s *Server) RegisterDaemon(instID flux.InstanceID, platform platform.Platform) (err error) {
	defer func(begin time.Time) {
		if err != nil {
			s.logger.Log("method", "RegisterDaemon", "err", err)
		}

		s.metrics.RegisterDaemonDuration.With(
			fluxmetrics.LabelInstanceID, fmt.Sprint(instID),
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
		s.metrics.ConnectedDaemons.Set(float64(atomic.AddInt32(&s.connected, -1)))
	}(time.Now())
	s.metrics.ConnectedDaemons.Set(float64(atomic.AddInt32(&s.connected, 1)))

	// Register the daemon with our message bus, waiting for it to be
	// closed. NB we cannot in general expect there to be a
	// configuration record for this instance; it may be connecting
	// before there is configuration supplied.
	done := make(chan error)
	s.messageBus.Subscribe(instID, s.instrumentPlatform(instID, platform), done)
	err = <-done
	close(done)
	return err
}

func (s *Server) instrumentPlatform(instID flux.InstanceID, p platform.Platform) platform.Platform {
	return &loggingPlatform{
		platform.Instrument(p, s.metrics.PlatformMetrics),
		log.NewContext(s.logger).With("instanceID", instID),
	}
}

func (s *Server) IsDaemonConnected(instID flux.InstanceID) error {
	return s.messageBus.Ping(instID)
}

type loggingPlatform struct {
	platform platform.Platform
	logger   log.Logger
}

func (p *loggingPlatform) AllServices(maybeNamespace string, ignored flux.ServiceIDSet) (ss []platform.Service, err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "AllServices", "error", err)
		}
	}()
	return p.platform.AllServices(maybeNamespace, ignored)
}

func (p *loggingPlatform) SomeServices(include []flux.ServiceID) (ss []platform.Service, err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "SomeServices", "error", err)
		}
	}()
	return p.platform.SomeServices(include)
}

func (p *loggingPlatform) Apply(defs []platform.ServiceDefinition) (err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "Apply", "error", err)
		}
	}()
	return p.platform.Apply(defs)
}

func (p *loggingPlatform) Ping() (err error) {
	defer func() {
		if err != nil {
			p.logger.Log("method", "Ping", "error", err)
		}
	}()
	return p.platform.Ping()
}
