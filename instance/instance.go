package instance

import (
	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/history"
	"github.com/weaveworks/flux/remote"
)

type Instancer interface {
	Get(inst flux.InstanceID) (*Instance, error)
}

type Instance struct {
	Platform remote.Platform
	Config   Configurer

	log.Logger
	history.EventReader
	history.EventWriter
}

func New(
	platform remote.Platform,
	config Configurer,
	logger log.Logger,
	events history.EventReader,
	eventlog history.EventWriter,
) *Instance {
	return &Instance{
		Platform:    platform,
		Config:      config,
		Logger:      logger,
		EventReader: events,
		EventWriter: eventlog,
	}
}
