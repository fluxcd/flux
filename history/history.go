package history

import (
	"io"
	"time"

	"github.com/weaveworks/flux"
)

type EventReadWriter interface {
	EventReader
	EventWriter
}

type EventWriter interface {
	// LogEvent records a message in the history of a service.
	LogEvent(flux.Event) error
}

type EventReader interface {
	// AllEvents returns a history for every service. Events must be
	// returned in descending timestamp order.
	AllEvents(time.Time, int64) ([]flux.Event, error)

	// EventsForService returns the history for a particular
	// service. Events must be returned in descending timestamp order.
	EventsForService(flux.ServiceID, time.Time, int64) ([]flux.Event, error)

	// GetEvent finds a single event, by ID.
	GetEvent(flux.EventID) (flux.Event, error)
}

type DB interface {
	LogEvent(flux.InstanceID, flux.Event) error
	AllEvents(flux.InstanceID, time.Time, int64) ([]flux.Event, error)
	EventsForService(flux.InstanceID, flux.ServiceID, time.Time, int64) ([]flux.Event, error)
	GetEvent(flux.EventID) (flux.Event, error)
	io.Closer
}
