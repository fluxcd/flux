package server

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/pkg/errors"

	"github.com/weaveworks/fluxy"
	"github.com/weaveworks/fluxy/history"
	"github.com/weaveworks/fluxy/instance"
	"github.com/weaveworks/fluxy/platform"
)

const (
	serviceAutomated   = "Automation enabled."
	serviceDeautomated = "Automation disabled."

	serviceLocked   = "Service locked."
	serviceUnlocked = "Service unlocked."

	secretReplacement = "******"
)

type Server struct {
	instancer   instance.Instancer
	messageBus  platform.MessageBus
	releaser    flux.ReleaseJobReadPusher
	maxPlatform chan struct{} // semaphore for concurrent calls to the platform
	metrics     Metrics
}

type Metrics struct {
	ListServicesDuration metrics.Histogram
	ListImagesDuration   metrics.Histogram
	HistoryDuration      metrics.Histogram
}

func New(
	instancer instance.Instancer,
	messageBus platform.MessageBus,
	releaser flux.ReleaseJobReadPusher,
	logger log.Logger,
	metrics Metrics,
) *Server {
	return &Server{
		instancer:   instancer,
		messageBus:  messageBus,
		releaser:    releaser,
		maxPlatform: make(chan struct{}, 8),
		metrics:     metrics,
	}
}

// The server methods are deliberately awkward, cobbled together from existing
// platform and registry APIs. I want to avoid changing those components until I
// get something working. There's also a lot of code duplication here for the
// same reason: let's not add abstraction until it's merged, or nearly so, and
// it's clear where the abstraction should exist.

func (s *Server) ListServices(inst flux.InstanceID, namespace string) (res []flux.ServiceStatus, err error) {
	defer func(begin time.Time) {
		s.metrics.ListServicesDuration.With(
			"namespace", namespace,
			"success", fmt.Sprint(err == nil),
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
			"success", fmt.Sprint(err == nil),
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
			"success", fmt.Sprint(err == nil),
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
			Stamp: event.Stamp,
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
	if err := inst.UpdateConfig(func(conf instance.Config) (instance.Config, error) {
		if serviceConf, found := conf.Services[service]; found {
			serviceConf.Automated = automated
			conf.Services[service] = serviceConf
		} else if automated {
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

func (s *Server) PostRelease(inst flux.InstanceID, spec flux.ReleaseJobSpec) (flux.ReleaseID, error) {
	return s.releaser.PutJob(inst, spec)
}

func (s *Server) GetRelease(inst flux.InstanceID, id flux.ReleaseID) (flux.ReleaseJob, error) {
	return s.releaser.GetJob(inst, id)
}

func (s *Server) GetConfig(instID flux.InstanceID, includeSecrets bool) (flux.InstanceConfig, error) {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return flux.InstanceConfig{}, err
	}
	fullConfig, err := inst.GetConfig()
	if err != nil {
		return flux.InstanceConfig{}, nil
	}

	config := fullConfig.Settings
	if !includeSecrets {
		removeSecrets(&config)
	}
	return config, nil
}

func (s *Server) SetConfig(instID flux.InstanceID, updates flux.InstanceConfig) error {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return err
	}
	return inst.UpdateConfig(applyConfigUpdates(updates))
}

func removeSecrets(config *flux.InstanceConfig) {
	for _, auth := range config.Registry.Auths {
		auth.Auth = secretReplacement
	}
	if config.Git.Key != "" {
		config.Git.Key = secretReplacement
	}
}

func applyConfigUpdates(updates flux.InstanceConfig) instance.UpdateFunc {
	return func(config instance.Config) (instance.Config, error) {
		config.Settings = updates
		return config, nil
	}
}

// Daemon handles a daemon connection. It blocks until the daemon has been
// disconnected.
//
// There are two conditions where we need to close and cleanup either the
// server has initiated a close (due to another client showing up) or the
// client has disconnected.
//
// If the server has initiated a close (due to another client showing up),
// we should close the other client's respective blocking goroutine.
//
// If the client has disconnected, there is no way to detect this in go,
// aside from just trying to connection. Therefore, the server will get an
// error when we try to use the client. We rely on that to break us out of
// the Daemon method.
func (s *Server) Daemon(instID flux.InstanceID, platform platform.Platform) error {
	// Register the daemon with our message bus, waiting for it to be closed
	return s.messageBus.Subscribe(instID, platform)
}
