package server

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/pkg/errors"

	"github.com/weaveworks/fluxy"
	"github.com/weaveworks/fluxy/helper"
	"github.com/weaveworks/fluxy/history"
	"github.com/weaveworks/fluxy/instance"
	"github.com/weaveworks/fluxy/platform/kubernetes"
	"github.com/weaveworks/fluxy/registry"
)

const (
	serviceLocked   = "Service locked."
	serviceUnlocked = "Service unlocked."

	hardwiredInstance = "DEFAULT"
)

type server struct {
	helper      *helper.Helper
	releaser    flux.ReleaseJobReadPusher
	automator   Automator
	eventReader history.EventReader
	eventWriter history.EventWriter
	instanceDB  instance.DB
	maxPlatform chan struct{} // semaphore for concurrent calls to the platform
	metrics     Metrics
}

type Automator interface {
	Automate(instanceID flux.InstanceID, namespace, service string) error
	Deautomate(instanceID flux.InstanceID, namespace, service string) error
	IsAutomated(instanceID flux.InstanceID, namespace, service string) bool
}

type Metrics struct {
	ListServicesDuration metrics.Histogram
	ListImagesDuration   metrics.Histogram
	HistoryDuration      metrics.Histogram
}

func New(
	platform *kubernetes.Cluster,
	registry *registry.Client,
	releaser flux.ReleaseJobReadPusher,
	automator Automator,
	eventReader history.EventReader,
	eventWriter history.EventWriter,
	instanceDB instance.DB,
	logger log.Logger,
	metrics Metrics,
	helperDuration metrics.Histogram,
) flux.Service {
	return &server{
		helper:      helper.New(platform, registry, logger, helperDuration),
		releaser:    releaser,
		automator:   automator,
		eventReader: eventReader,
		eventWriter: eventWriter,
		instanceDB:  instanceDB,
		maxPlatform: make(chan struct{}, 8),
		metrics:     metrics,
	}
}

// The server methods are deliberately awkward, cobbled together from existing
// platform and registry APIs. I want to avoid changing those components until I
// get something working. There's also a lot of code duplication here for the
// same reason: let's not add abstraction until it's merged, or nearly so, and
// it's clear where the abstraction should exist.

func (s *server) ListServices(inst flux.InstanceID, namespace string) (res []flux.ServiceStatus, err error) {
	defer func(begin time.Time) {
		s.metrics.ListServicesDuration.With(
			"namespace", namespace,
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	var serviceIDs []flux.ServiceID
	if namespace == "" {
		serviceIDs, err = s.helper.AllServices()
	} else {
		serviceIDs, err = s.helper.NamespaceServices(namespace)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "fetching services for namespace %s on the platform", namespace)
	}

	config, err := s.instanceDB.GetConfig(hardwiredInstance)
	if err != nil {
		return nil, errors.Wrapf(err, "getting config for %s", hardwiredInstance)
	}

	var (
		statusc = make(chan flux.ServiceStatus)
		errc    = make(chan error)
	)
	for _, serviceID := range serviceIDs {
		go func(serviceID flux.ServiceID) {
			s.maxPlatform <- struct{}{}
			defer func() { <-s.maxPlatform }()

			c, err := s.containersFor(serviceID, false)
			if err != nil {
				errc <- errors.Wrapf(err, "fetching containers for %s", serviceID)
				return
			}

			namespace, service := serviceID.Components()
			platformSvc, err := s.helper.PlatformService(namespace, service)
			if err != nil {
				errc <- errors.Wrapf(err, "getting platform service %s", serviceID)
				return
			}

			statusc <- flux.ServiceStatus{
				ID:         serviceID,
				Containers: c,
				Status:     platformSvc.Status,
				Automated:  config.Services[serviceID].Automated,
				Locked:     config.Services[serviceID].Locked,
			}
		}(serviceID)
	}
	for i := 0; i < len(serviceIDs); i++ {
		select {
		case err := <-errc:
			s.helper.Log("err", err)
		case status := <-statusc:
			res = append(res, status)
		}
	}
	return res, nil
}

func (s *server) ListImages(inst flux.InstanceID, spec flux.ServiceSpec) (res []flux.ImageStatus, err error) {
	defer func(begin time.Time) {
		s.metrics.ListImagesDuration.With(
			"service_spec", fmt.Sprint(spec),
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	serviceIDs, err := func() ([]flux.ServiceID, error) {
		if spec == flux.ServiceSpecAll {
			return s.helper.AllServices()
		}
		id, err := flux.ParseServiceID(string(spec))
		return []flux.ServiceID{id}, err
	}()
	if err != nil {
		return nil, errors.Wrapf(err, "fetching service ID(s)")
	}

	var (
		statusc = make(chan flux.ImageStatus)
		errc    = make(chan error)
	)
	for _, serviceID := range serviceIDs {
		go func(serviceID flux.ServiceID) {
			s.maxPlatform <- struct{}{}
			defer func() { <-s.maxPlatform }()

			c, err := s.containersFor(serviceID, true)
			if err != nil {
				errc <- errors.Wrapf(err, "fetching containers for %s", serviceID)
				return
			}

			statusc <- flux.ImageStatus{
				ID:         serviceID,
				Containers: c,
			}
		}(serviceID)
	}
	for i := 0; i < len(serviceIDs); i++ {
		select {
		case err := <-errc:
			s.helper.Log("err", err)
		case status := <-statusc:
			res = append(res, status)
		}
	}

	return res, nil
}

func (s *server) History(inst flux.InstanceID, spec flux.ServiceSpec) (res []flux.HistoryEntry, err error) {
	defer func(begin time.Time) {
		s.metrics.HistoryDuration.With(
			"service_spec", fmt.Sprint(spec),
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	var events []history.Event
	if spec == flux.ServiceSpecAll {
		events, err = s.eventReader.AllEvents()
		if err != nil {
			return nil, errors.Wrap(err, "fetching all history events")
		}
	} else {
		id, err := flux.ParseServiceID(string(spec))
		if err != nil {
			return nil, errors.Wrapf(err, "parsing service ID from spec %s", spec)
		}

		namespace, service := id.Components()
		events, err = s.eventReader.EventsForService(namespace, service)
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

func (s *server) Automate(inst flux.InstanceID, service flux.ServiceID) error {
	ns, svc := service.Components()
	return s.automator.Automate(inst, ns, svc)
}

func (s *server) Deautomate(inst flux.InstanceID, service flux.ServiceID) error {
	ns, svc := service.Components()
	return s.automator.Deautomate(inst, ns, svc)
}

func (s *server) Lock(inst flux.InstanceID, service flux.ServiceID) error {
	ns, svc := service.Components()
	s.eventWriter.LogEvent(ns, svc, serviceLocked)
	return s.recordLock(service, true)
}

func (s *server) Unlock(inst flux.InstanceID, service flux.ServiceID) error {
	ns, svc := service.Components()
	s.eventWriter.LogEvent(ns, svc, serviceUnlocked)
	return s.recordLock(service, false)
}

func (s *server) recordLock(service flux.ServiceID, locked bool) error {
	if err := s.instanceDB.UpdateConfig(hardwiredInstance, func(conf instance.Config) (instance.Config, error) {
		if serviceConf, found := conf.Services[service]; found {
			serviceConf.Locked = locked
			conf.Services[service] = serviceConf
		} else {
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

func (s *server) PostRelease(inst flux.InstanceID, spec flux.ReleaseJobSpec) (flux.ReleaseID, error) {
	return s.releaser.PutJob(spec)
}

func (s *server) GetRelease(inst flux.InstanceID, id flux.ReleaseID) (flux.ReleaseJob, error) {
	return s.releaser.GetJob(id)
}

func (s *server) containersFor(id flux.ServiceID, includeAvailable bool) (res []flux.Container, _ error) {
	namespace, service := id.Components()
	containers, err := s.helper.PlatformContainersFor(namespace, service)
	if err != nil {
		return nil, errors.Wrapf(err, "fetching containers for %s", id)
	}

	var errs compositeError
	for _, container := range containers {
		imageID := flux.ParseImageID(container.Image)

		// We may not be able to get image info from the repository,
		// but it's still worthwhile returning what we know.
		current := flux.ImageDescription{ID: imageID}
		var available []flux.ImageDescription

		if includeAvailable {
			imageRepo, err := s.helper.RegistryGetRepository(imageID.Repository())
			if err != nil {
				errs = append(errs, errors.Wrapf(err, "fetching image repo for %s", imageID))
			} else {
				for _, image := range imageRepo.Images {
					description := flux.ImageDescription{
						ID:        flux.ParseImageID(image.String()),
						CreatedAt: image.CreatedAt,
					}
					available = append(available, description)
					if image.String() == container.Image {
						current = description
					}
				}
			}
		}
		res = append(res, flux.Container{
			Name:      container.Name,
			Current:   current,
			Available: available,
		})
	}

	if len(errs) > 0 {
		return res, errors.Wrap(errs, "one or more errors fetching image repos")
	}
	return res, nil
}

type compositeError []error

func (e compositeError) Error() string {
	msgs := make([]string, len(e))
	for i, err := range e {
		msgs[i] = err.Error()
	}
	return strings.Join(msgs, "; ")
}
