package server

import (
	"sync/atomic"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/update"
)

const (
	serviceAutomated   = "Automation enabled."
	serviceDeautomated = "Automation disabled."

	serviceLocked   = "Service locked."
	serviceUnlocked = "Service unlocked."
)

type Server struct {
	version     string
	instancer   instance.Instancer
	config      instance.DB
	messageBus  remote.MessageBus
	logger      log.Logger
	maxPlatform chan struct{} // semaphore for concurrent calls to the platform
	connected   int32
}

func New(
	version string,
	instancer instance.Instancer,
	config instance.DB,
	messageBus remote.MessageBus,
	logger log.Logger,
) *Server {
	connectedDaemons.Set(0)
	return &Server{
		version:     version,
		instancer:   instancer,
		config:      config,
		messageBus:  messageBus,
		logger:      logger,
		maxPlatform: make(chan struct{}, 8),
	}
}

func (s *Server) Status(instID flux.InstanceID) (res flux.Status, err error) {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return res, errors.Wrapf(err, "getting instance")
	}

	res.Fluxsvc = flux.FluxsvcStatus{Version: s.version}
	res.Fluxd.Version, err = inst.Platform.Version()
	res.Fluxd.Connected = (err == nil)
	_, err = inst.Platform.SyncStatus("HEAD")
	if err != nil {
		res.Git.Error = err.Error()
	} else {
		res.Git.Configured = true
	}

	return res, nil
}

func (s *Server) ListServices(instID flux.InstanceID, namespace string) (res []flux.ServiceStatus, err error) {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return nil, errors.Wrapf(err, "getting instance")
	}

	services, err := inst.Platform.ListServices(namespace)
	if err != nil {
		return nil, errors.Wrap(err, "getting services from platform")
	}
	return services, nil
}

func (s *Server) ListImages(instID flux.InstanceID, spec flux.ServiceSpec) (res []flux.ImageStatus, err error) {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return nil, errors.Wrapf(err, "getting instance "+string(instID))
	}
	return inst.Platform.ListImages(spec)
}

func (s *Server) UpdateImages(instID flux.InstanceID, spec flux.ReleaseSpec) (job.ID, error) {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return "", errors.Wrapf(err, "getting instance "+string(instID))
	}
	return inst.Platform.UpdateManifests(update.Spec{Type: update.Images, Spec: spec})
}

func (s *Server) UpdatePolicies(instID flux.InstanceID, updates flux.PolicyUpdates) (job.ID, error) {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return "", errors.Wrapf(err, "getting instance "+string(instID))
	}

	return inst.Platform.UpdateManifests(update.Spec{Type: update.Policy, Spec: updates})
}

func (s *Server) SyncNotify(instID flux.InstanceID) (err error) {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return errors.Wrapf(err, "getting instance "+string(instID))
	}
	return inst.Platform.SyncNotify()
}

func (s *Server) SyncStatus(instID flux.InstanceID, ref string) (res []string, err error) {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return nil, errors.Wrapf(err, "getting instance "+string(instID))
	}

	return inst.Platform.SyncStatus(ref)
}

// LogEvent stores an event in the instance's history.
func (s *Server) LogEvent(instID flux.InstanceID, event flux.Event) error {
	helper, err := s.instancer.Get(instID)
	if err != nil {
		return errors.Wrapf(err, "getting instance")
	}
	return helper.LogEvent(event)
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

func (s *Server) GetConfig(instID flux.InstanceID, fingerprint string) (flux.InstanceConfig, error) {
	fullConfig, err := s.config.GetConfig(instID)
	if err != nil {
		return flux.InstanceConfig{}, err
	}

	config := flux.InstanceConfig(fullConfig.Settings)

	return config, nil
}

func (s *Server) SetConfig(instID flux.InstanceID, updates flux.UnsafeInstanceConfig) error {
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

	return s.config.UpdateConfig(instID, applyConfigUpdates(patchedConfig))
}

func applyConfigUpdates(updates flux.UnsafeInstanceConfig) instance.UpdateFunc {
	return func(config instance.Config) (instance.Config, error) {
		config.Settings = updates
		return config, nil
	}
}

// FIXME this will have to be done differently; also it's part of the
// service iface
func (s *Server) GenerateDeployKey(instID flux.InstanceID) error {
	// Generate new key
	unsafePrivateKey, err := git.NewKeyGenerator().Generate()
	if err != nil {
		return err
	}

	// Get current config
	cfg, err := s.GetConfig(instID, "")
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
func (s *Server) RegisterDaemon(instID flux.InstanceID, platform remote.Platform) (err error) {
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

func (s *Server) Export(instID flux.InstanceID) (res []byte, err error) {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return res, errors.Wrapf(err, "getting instance")
	}

	res, err = inst.Platform.Export()
	if err != nil {
		return res, errors.Wrapf(err, "exporting %s", instID)
	}

	return res, nil
}

func (s *Server) instrumentPlatform(instID flux.InstanceID, p remote.Platform) remote.Platform {
	return &remote.ErrorLoggingPlatform{
		remote.Instrument(p),
		log.NewContext(s.logger).With("instanceID", instID),
	}
}

func (s *Server) IsDaemonConnected(instID flux.InstanceID) error {
	return s.messageBus.Ping(instID)
}
