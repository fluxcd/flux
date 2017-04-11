package server

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/jobs"
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
	version     string
	webhookURL  string
	instancer   instance.Instancer
	config      instance.DB
	messageBus  platform.MessageBus
	jobs        jobs.JobStore
	idMapper    instance.IDMapper
	logger      log.Logger
	maxPlatform chan struct{} // semaphore for concurrent calls to the platform
	connected   int32
}

func New(
	version, webhookURL string,
	instancer instance.Instancer,
	config instance.DB,
	messageBus platform.MessageBus,
	jobs jobs.JobStore,
	idMapper instance.IDMapper,
	logger log.Logger,
) *Server {
	connectedDaemons.Set(0)
	return &Server{
		version:     version,
		instancer:   instancer,
		config:      config,
		messageBus:  messageBus,
		jobs:        jobs,
		idMapper:    idMapper,
		logger:      logger,
		maxPlatform: make(chan struct{}, 8),
	}
}

// The server methods are deliberately awkward, cobbled together from existing
// platform and registry APIs. I want to avoid changing those components until I
// get something working. There's also a lot of code duplication here for the
// same reason: let's not add abstraction until it's merged, or nearly so, and
// it's clear where the abstraction should exist.

func (s *Server) WebhookURL(inst flux.InstanceID) (string, error) {
	external, err := s.idMapper.ExternalInstanceID(inst)
	if err != nil {
		return "", errors.Wrap(err, "fetching instance id")
	}
	return filepath.Join(s.webhookURL, external), nil
}

func (s *Server) Status(inst flux.InstanceID) (res flux.Status, err error) {
	helper, err := s.instancer.Get(inst)
	if err != nil {
		return res, errors.Wrapf(err, "getting instance")
	}

	config, err := helper.GetConfig()
	if err != nil {
		return res, errors.Wrapf(err, "getting config for %s", inst)
	}
	res.Git.Configured = config.Settings.Git.URL != "" && config.Settings.Git.Key != ""

	if _, err := helper.ConfigRepo().Clone(); err != nil {
		// Remove \r, so it prints as a yaml block
		res.Git.Error = strings.Replace(err.Error(), "\r", "", -1)
	}

	res.Fluxsvc = flux.FluxsvcStatus{Version: s.version}
	res.Fluxd.Version, err = helper.Version()
	res.Fluxd.Connected = (err == nil)

	return res, nil
}

func (s *Server) ListServices(inst flux.InstanceID, namespace string) (res []flux.ServiceStatus, err error) {
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
		id, _ := flux.ParseImageID(c.Image)
		res[i] = flux.Container{
			Name: c.Name,
			Current: flux.ImageDescription{
				ID: id,
			},
		}
	}
	return res
}

func (s *Server) ListImages(inst flux.InstanceID, spec flux.ServiceSpec) (res []flux.ImageStatus, err error) {
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
		id, _ := flux.ParseImageID(c.Image)
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

func (s *Server) History(inst flux.InstanceID, spec flux.ServiceSpec, before time.Time, limit int64) (res []flux.HistoryEntry, err error) {
	helper, err := s.instancer.Get(inst)
	if err != nil {
		return nil, errors.Wrapf(err, "getting instance")
	}

	var events []flux.Event
	if spec == flux.ServiceSpecAll {
		events, err = helper.AllEvents(before, limit)
		if err != nil {
			return nil, errors.Wrap(err, "fetching all history events")
		}
	} else {
		id, err := flux.ParseServiceID(string(spec))
		if err != nil {
			return nil, errors.Wrapf(err, "parsing service ID from spec %s", spec)
		}

		events, err = helper.EventsForService(id, before, limit)
		if err != nil {
			return nil, errors.Wrapf(err, "fetching history events for %s", id)
		}
	}

	res = make([]flux.HistoryEntry, len(events))
	for i, event := range events {
		res[i] = flux.HistoryEntry{
			Stamp: &events[i].StartedAt,
			Type:  "v0",
			Data:  event.String(),
			Event: &events[i],
		}
	}

	return res, nil
}

func (s *Server) Automate(instID flux.InstanceID, service flux.ServiceID) error {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := inst.LogEvent(flux.Event{
		ServiceIDs: []flux.ServiceID{service},
		Type:       flux.EventAutomate,
		StartedAt:  now,
		EndedAt:    now,
		LogLevel:   flux.LogLevelInfo,
	}); err != nil {
		return err
	}
	return recordAutomated(inst, service, true)
}

func (s *Server) Deautomate(instID flux.InstanceID, service flux.ServiceID) error {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := inst.LogEvent(flux.Event{
		ServiceIDs: []flux.ServiceID{service},
		Type:       flux.EventDeautomate,
		StartedAt:  now,
		EndedAt:    now,
		LogLevel:   flux.LogLevelInfo,
	}); err != nil {
		return err
	}
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
	now := time.Now().UTC()
	if err := inst.LogEvent(flux.Event{
		ServiceIDs: []flux.ServiceID{service},
		Type:       flux.EventLock,
		StartedAt:  now,
		EndedAt:    now,
		LogLevel:   flux.LogLevelInfo,
	}); err != nil {
		return err
	}
	return recordLock(inst, service, true)
}

func (s *Server) Unlock(instID flux.InstanceID, service flux.ServiceID) error {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := inst.LogEvent(flux.Event{
		ServiceIDs: []flux.ServiceID{service},
		Type:       flux.EventUnlock,
		StartedAt:  now,
		EndedAt:    now,
		LogLevel:   flux.LogLevelInfo,
	}); err != nil {
		return err
	}
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
		return flux.InstanceConfig{}, err
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

func (s *Server) PatchConfig(instID flux.InstanceID, patch flux.ConfigPatch) error {
	fullConfig, err := s.config.GetConfig(instID)
	if err != nil {
		return errors.Wrap(err, "unable to get config")
	}

	patchedConfig, err := fullConfig.Settings.Patch(patch)
	if err != nil {
		return errors.Wrap(err, "unable to apply patch")
	}

	if _, err := registry.CredentialsFromConfig(patchedConfig); err != nil {
		return errors.Wrap(err, "invalid registry credentials")
	}
	return s.config.UpdateConfig(instID, applyConfigUpdates(patchedConfig))
}

func applyConfigUpdates(updates flux.UnsafeInstanceConfig) instance.UpdateFunc {
	return func(config instance.Config) (instance.Config, error) {
		config.Settings = updates
		return config, nil
	}
}

func (s *Server) GenerateDeployKey(instID flux.InstanceID) error {
	// Generate new key
	unsafePrivateKey, err := git.NewKeyGenerator().Generate()
	if err != nil {
		return err
	}

	// Get current config
	cfg, err := s.GetConfig(instID)
	if err != nil {
		return err
	}
	cfg.Git.Key = string(unsafePrivateKey)

	// Set new config
	return s.config.UpdateConfig(instID, applyConfigUpdates(flux.UnsafeInstanceConfig(cfg)))
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
	defer func() {
		if err != nil {
			s.logger.Log("method", "RegisterDaemon", "err", err)
		}
		connectedDaemons.Set(float64(atomic.AddInt32(&s.connected, -1)))
	}()
	connectedDaemons.Set(float64(atomic.AddInt32(&s.connected, 1)))

	// Register the daemon with our message bus, waiting for it to be
	// closed. NB we cannot in general expect there to be a
	// configuration record for this instance; it may be connecting
	// before there is configuration supplied.
	done := make(chan error)
	s.messageBus.Subscribe(instID, s.instrumentPlatform(instID, platform), done)
	err = <-done
	return err
}

func (s *Server) Export(inst flux.InstanceID) (res []byte, err error) {
	helper, err := s.instancer.Get(inst)
	if err != nil {
		return res, errors.Wrapf(err, "getting instance")
	}

	res, err = helper.Export()
	if err != nil {
		return res, errors.Wrapf(err, "exporting %s", inst)
	}

	return res, nil
}

func (s *Server) instrumentPlatform(instID flux.InstanceID, p platform.Platform) platform.Platform {
	return &loggingPlatform{
		platform.Instrument(p),
		log.NewContext(s.logger).With("instanceID", instID),
	}
}

func (s *Server) IsDaemonConnected(instID flux.InstanceID) error {
	return s.messageBus.Ping(instID)
}

func (s *Server) Watch(instID flux.InstanceID) (string, error) {
	// Get current config
	cfg, err := s.GetConfig(instID)
	if err != nil {
		return "", err
	}
	cfg.Watching = true

	// TODO: Sync the repo, and apply the state immediately!

	// Set new config
	if err := s.config.UpdateConfig(instID, applyConfigUpdates(flux.UnsafeInstanceConfig(cfg))); err != nil {
		return "", err
	}
	return s.WebhookURL(instID)
}

func (s *Server) Unwatch(instID flux.InstanceID) error {
	// Get current config
	cfg, err := s.GetConfig(instID)
	if err != nil {
		return err
	}
	cfg.Watching = false

	// Set new config
	return s.config.UpdateConfig(instID, applyConfigUpdates(flux.UnsafeInstanceConfig(cfg)))
}

func (s *Server) RepoUpdate(externalInstanceID string) error {
	inst, err := s.idMapper.InternalInstanceID(externalInstanceID)
	if err != nil {
		return err
	}

	_, err = s.jobs.PutJob(inst, jobs.Job{
		Queue: jobs.SyncJob,
		// Key stops us getting two queued jobs for the same instance
		Key: strings.Join([]string{
			jobs.SyncJob,
			string(inst),
		}, "|"),
		Method:   jobs.SyncJob,
		Priority: jobs.PriorityBackground,
		Params: jobs.SyncJobParams{
			InstanceID: inst,
		},
	})
	return err
}
