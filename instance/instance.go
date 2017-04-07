package instance

import (
	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/history"
	"github.com/weaveworks/flux/platform"
)

type Instancer interface {
	Get(inst flux.InstanceID) (*Instance, error)
}

type Instance struct {
	Platform platform.Platform
	Config   Configurer

	log.Logger
	history.EventReader
	history.EventWriter
}

func New(
	platform platform.Platform,
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
