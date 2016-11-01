package instance

import (
	"errors"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"

	"github.com/weaveworks/fluxy"
	"github.com/weaveworks/fluxy/git"
	"github.com/weaveworks/fluxy/history"
	"github.com/weaveworks/fluxy/platform"
	"github.com/weaveworks/fluxy/registry"
)

// StandaloneInstancer is the instancer for standalone mode
type StandaloneInstancer struct {
	Instance     flux.InstanceID
	Connecter    platform.Connecter
	Registry     *registry.Client
	Config       Configurer
	GitRepo      git.Repo
	EventReader  history.EventReader
	EventWriter  history.EventWriter
	BaseLogger   log.Logger
	BaseDuration metrics.Histogram
}

func (s StandaloneInstancer) Get(inst flux.InstanceID) (*Instance, error) {
	if inst != s.Instance {
		return nil, errors.New("cannot find instance with ID: " + string(inst))
	}
	platform, err := s.Connecter.Connect(inst)
	if err != nil {
		return nil, errors.New("cannot get platform for instance")
	}
	return New(
		platform,
		s.Registry,
		s.Config,
		s.GitRepo,
		log.NewContext(s.BaseLogger).With("instanceID", s.Instance),
		s.BaseDuration,
		s.EventReader,
		s.EventWriter,
	), nil
}
