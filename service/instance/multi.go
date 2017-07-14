package instance

import (
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux/history"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/service"
)

type MultitenantInstancer struct {
	DB        DB
	Connecter remote.Connecter
	Logger    log.Logger
	History   history.DB
}

func (m *MultitenantInstancer) Get(instanceID service.InstanceID) (*Instance, error) {
	// Platform interface for this instance
	platform, err := m.Connecter.Connect(instanceID)
	if err != nil {
		return nil, errors.Wrap(err, "connecting to platform")
	}

	// Logger specialised to this instance
	instanceLogger := log.NewContext(m.Logger).With("instanceID", instanceID)

	// Events for this instance
	eventRW := EventReadWriter{instanceID, m.History}

	// Configuration for this instance
	config := configurer{instanceID, m.DB}

	return New(
		platform,
		config,
		instanceLogger,
		eventRW,
		eventRW,
	), nil
}
