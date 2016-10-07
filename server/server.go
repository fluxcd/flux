package server

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/pkg/errors"

	"github.com/weaveworks/fluxy"
	"github.com/weaveworks/fluxy/history"
	"github.com/weaveworks/fluxy/instance"
)

const (
	serviceLocked   = "Service locked."
	serviceUnlocked = "Service unlocked."
)

type server struct {
	instancer   instance.Instancer
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
	releaser flux.ReleaseJobReadPusher,
	logger log.Logger,
	metrics Metrics,
) flux.Service {
	return &server{
		instancer:   instancer,
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

func (s *server) ListServices(inst flux.InstanceID, namespace string) (res []flux.ServiceStatus, err error) {
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

	var serviceIDs []flux.ServiceID
	if namespace == "" {
		serviceIDs, err = helper.AllServices()
	} else {
		serviceIDs, err = helper.NamespaceServices(namespace)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "fetching services for namespace %s on the platform", namespace)
	}

	config, err := helper.GetConfig()
	if err != nil {
		return nil, errors.Wrapf(err, "getting config for %s", inst)
	}

	var (
		statusc = make(chan flux.ServiceStatus)
		errc    = make(chan error)
	)
	for _, serviceID := range serviceIDs {
		go func(serviceID flux.ServiceID) {
			s.maxPlatform <- struct{}{}
			defer func() { <-s.maxPlatform }()

			c, err := containersFor(helper, serviceID, false)
			if err != nil {
				errc <- errors.Wrapf(err, "fetching containers for %s", serviceID)
				return
			}

			namespace, service := serviceID.Components()
			platformSvc, err := helper.PlatformService(namespace, service)
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
			helper.Log("err", err)
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

	helper, err := s.instancer.Get(inst)
	if err != nil {
		return nil, errors.Wrapf(err, "getting instance")
	}

	serviceIDs, err := func() ([]flux.ServiceID, error) {
		if spec == flux.ServiceSpecAll {
			return helper.AllServices()
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

			c, err := containersFor(helper, serviceID, true)
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
			helper.Log("err", err)
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

func (s *server) Automate(instID flux.InstanceID, service flux.ServiceID) error {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return err
	}
	return recordAutomated(inst, service, true)
}

func (s *server) Deautomate(instID flux.InstanceID, service flux.ServiceID) error {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return err
	}
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

func (s *server) Lock(instID flux.InstanceID, service flux.ServiceID) error {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return err
	}
	ns, svc := service.Components()
	inst.LogEvent(ns, svc, serviceLocked)
	return recordLock(inst, service, true)
}

func (s *server) Unlock(instID flux.InstanceID, service flux.ServiceID) error {
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

func (s *server) PostRelease(inst flux.InstanceID, spec flux.ReleaseJobSpec) (flux.ReleaseID, error) {
	return s.releaser.PutJob(inst, spec)
}

func (s *server) GetRelease(inst flux.InstanceID, id flux.ReleaseID) (flux.ReleaseJob, error) {
	return s.releaser.GetJob(inst, id)
}

func containersFor(helper *instance.Instance, id flux.ServiceID, includeAvailable bool) (res []flux.Container, _ error) {
	namespace, service := id.Components()
	containers, err := helper.PlatformContainersFor(namespace, service)
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
			imageRepo, err := helper.RegistryGetRepository(imageID.Repository())
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
