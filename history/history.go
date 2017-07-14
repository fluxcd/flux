package history

import (
	"io"
	"time"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/service"
)

type EventReadWriter interface {
	EventReader
	EventWriter
}

type EventWriter interface {
	// LogEvent records a message in the history of a service.
	LogEvent(Event) error
}

type EventReader interface {
	// AllEvents returns a history for every service. Events must be
	// returned in descending timestamp order.
	AllEvents(time.Time, int64, time.Time) ([]Event, error)

	// EventsForService returns the history for a particular
	// service. Events must be returned in descending timestamp order.
	EventsForService(flux.ServiceID, time.Time, int64, time.Time) ([]Event, error)

	// GetEvent finds a single event, by ID.
	GetEvent(EventID) (Event, error)
}

type DB interface {
	LogEvent(service.InstanceID, Event) error
	AllEvents(service.InstanceID, time.Time, int64, time.Time) ([]Event, error)
	EventsForService(service.InstanceID, flux.ServiceID, time.Time, int64, time.Time) ([]Event, error)
	GetEvent(EventID) (Event, error)
	io.Closer
}
