package server

import (
	"sync/atomic"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/history"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/notifications"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/service"
	"github.com/weaveworks/flux/service/instance"
	"github.com/weaveworks/flux/ssh"
	"github.com/weaveworks/flux/update"
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

func (s *Server) Status(instID service.InstanceID) (res service.Status, err error) {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return res, errors.Wrapf(err, "getting instance")
	}

	res.Fluxsvc = service.FluxsvcStatus{Version: s.version}

	config, err := inst.Config.Get()
	if err != nil {
		return res, err
	}

	res.Fluxd.Last = config.Connection.Last
	// DOn't bother trying to get information from the daemon if we
	// haven't recorded it as connected
	if config.Connection.Connected {
		res.Fluxd.Connected = true
		res.Fluxd.Version, err = inst.Platform.Version()
		if err != nil {
			return res, err
		}

		res.Git.Config, err = inst.Platform.GitRepoConfig(false)
		if err != nil {
			return res, err
		}

		_, err = inst.Platform.SyncStatus("HEAD")
		if err != nil {
			res.Git.Error = err.Error()
		} else {
			res.Git.Configured = true
		}
	}

	return res, nil
}

func (s *Server) ListServices(instID service.InstanceID, namespace string) (res []flux.ServiceStatus, err error) {
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

func (s *Server) ListImages(instID service.InstanceID, spec update.ServiceSpec) (res []flux.ImageStatus, err error) {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return nil, errors.Wrapf(err, "getting instance "+string(instID))
	}
	return inst.Platform.ListImages(spec)
}

func (s *Server) UpdateImages(instID service.InstanceID, spec update.ReleaseSpec, cause update.Cause) (job.ID, error) {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return "", errors.Wrapf(err, "getting instance "+string(instID))
	}
	return inst.Platform.UpdateManifests(update.Spec{Type: update.Images, Cause: cause, Spec: spec})
}

func (s *Server) UpdatePolicies(instID service.InstanceID, updates policy.Updates, cause update.Cause) (job.ID, error) {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return "", errors.Wrapf(err, "getting instance "+string(instID))
	}

	return inst.Platform.UpdateManifests(update.Spec{Type: update.Policy, Cause: cause, Spec: updates})
}

func (s *Server) SyncNotify(instID service.InstanceID) (err error) {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return errors.Wrapf(err, "getting instance "+string(instID))
	}
	return inst.Platform.SyncNotify()
}

func (s *Server) JobStatus(instID service.InstanceID, jobID job.ID) (res job.Status, err error) {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return job.Status{}, errors.Wrapf(err, "getting instance "+string(instID))
	}

	return inst.Platform.JobStatus(jobID)
}

func (s *Server) SyncStatus(instID service.InstanceID, ref string) (res []string, err error) {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return nil, errors.Wrapf(err, "getting instance "+string(instID))
	}

	return inst.Platform.SyncStatus(ref)
}

// LogEvent receives events from fluxd and pushes events to the history
// db and a slack notification
func (s *Server) LogEvent(instID service.InstanceID, e history.Event) error {
	s.logger.Log("method", "LogEvent", "instance", instID, "event", e)
	helper, err := s.instancer.Get(instID)
	if err != nil {
		return errors.Wrapf(err, "getting instance")
	}

	// Log event in history first. This is less likely to fail
	err = helper.LogEvent(e)
	if err != nil {
		return errors.Wrapf(err, "logging event")
	}

	cfg, err := helper.Config.Get()
	if err != nil {
		return errors.Wrapf(err, "getting config")
	}
	err = notifications.Event(cfg, e)
	if err != nil {
		return errors.Wrapf(err, "sending notifications")
	}
	return nil
}

func (s *Server) History(inst service.InstanceID, spec update.ServiceSpec, before time.Time, limit int64, after time.Time) (res []history.Entry, err error) {
	helper, err := s.instancer.Get(inst)
	if err != nil {
		return nil, errors.Wrapf(err, "getting instance")
	}

	var events []history.Event
	if spec == update.ServiceSpecAll {
		events, err = helper.AllEvents(before, limit, after)
		if err != nil {
			return nil, errors.Wrap(err, "fetching all history events")
		}
	} else {
		id, err := flux.ParseServiceID(string(spec))
		if err != nil {
			return nil, errors.Wrapf(err, "parsing service ID from spec %s", spec)
		}

		events, err = helper.EventsForService(id, before, limit, after)
		if err != nil {
			return nil, errors.Wrapf(err, "fetching history events for %s", id)
		}
	}

	res = make([]history.Entry, len(events))
	for i, event := range events {
		res[i] = history.Entry{
			Stamp: &events[i].StartedAt,
			Type:  "v0",
			Data:  event.String(),
			Event: &events[i],
		}
	}

	return res, nil
}

func (s *Server) GetConfig(instID service.InstanceID, fingerprint string) (service.InstanceConfig, error) {
	fullConfig, err := s.config.GetConfig(instID)
	if err != nil {
		return service.InstanceConfig{}, err
	}

	config := service.InstanceConfig(fullConfig.Settings)

	return config, nil
}

func (s *Server) SetConfig(instID service.InstanceID, updates service.InstanceConfig) error {
	return s.config.UpdateConfig(instID, applyConfigUpdates(updates))
}

func (s *Server) PatchConfig(instID service.InstanceID, patch service.ConfigPatch) error {
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

func applyConfigUpdates(updates service.InstanceConfig) instance.UpdateFunc {
	return func(config instance.Config) (instance.Config, error) {
		config.Settings = updates
		return config, nil
	}
}

func (s *Server) PublicSSHKey(instID service.InstanceID, regenerate bool) (ssh.PublicKey, error) {
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return ssh.PublicKey{}, errors.Wrapf(err, "getting instance "+string(instID))
	}

	gitRepoConfig, err := inst.Platform.GitRepoConfig(regenerate)
	if err != nil {
		return ssh.PublicKey{}, err
	}
	return gitRepoConfig.PublicSSHKey, nil
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
func (s *Server) RegisterDaemon(instID service.InstanceID, platform remote.Platform) (err error) {
	defer func() {
		if err != nil {
			s.logger.Log("method", "RegisterDaemon", "err", err)
		}
		connectedDaemons.Set(float64(atomic.AddInt32(&s.connected, -1)))
	}()
	connectedDaemons.Set(float64(atomic.AddInt32(&s.connected, 1)))

	// Record the time of connection in the "config"
	now := time.Now()
	s.config.UpdateConfig(instID, setConnectionTime(now))
	defer s.config.UpdateConfig(instID, setDisconnectedIf(now))

	// Register the daemon with our message bus, waiting for it to be
	// closed. NB we cannot in general expect there to be a
	// configuration record for this instance; it may be connecting
	// before there is configuration supplied.
	done := make(chan error)
	s.messageBus.Subscribe(instID, s.instrumentPlatform(instID, platform), done)
	err = <-done
	return err
}

func setConnectionTime(t time.Time) instance.UpdateFunc {
	return func(config instance.Config) (instance.Config, error) {
		config.Connection.Last = t
		config.Connection.Connected = true
		return config, nil
	}
}

// Only set the connection time if it's what you think it is (i.e., a
// kind of compare and swap). Used so that disconnecting doesn't zero
// the value set by another connection.
func setDisconnectedIf(t0 time.Time) instance.UpdateFunc {
	return func(config instance.Config) (instance.Config, error) {
		if config.Connection.Last.Equal(t0) {
			config.Connection.Connected = false
		}
		return config, nil
	}
}

func (s *Server) Export(instID service.InstanceID) (res []byte, err error) {
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

func (s *Server) instrumentPlatform(instID service.InstanceID, p remote.Platform) remote.Platform {
	return &remote.ErrorLoggingPlatform{
		remote.Instrument(p),
		log.NewContext(s.logger).With("instanceID", instID),
	}
}

func (s *Server) IsDaemonConnected(instID service.InstanceID) error {
	return s.messageBus.Ping(instID)
}
