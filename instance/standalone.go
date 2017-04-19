package instance

import (
	"errors"

	"github.com/go-kit/kit/log"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/history"
	"github.com/weaveworks/flux/remote"
)

// StandaloneInstancer is the instancer for standalone mode
type StandaloneInstancer struct {
	Instance    flux.InstanceID
	Connecter   remote.Connecter
	Config      Configurer
	EventReader history.EventReader
	EventWriter history.EventWriter
	BaseLogger  log.Logger
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
		s.Config,
		log.NewContext(s.BaseLogger).With("instanceID", s.Instance),
		s.EventReader,
		s.EventWriter,
	), nil
}
