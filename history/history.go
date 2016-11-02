package history

import (
	"io"
	"time"

	"github.com/weaveworks/flux"
)

type Event struct {
	Service, Msg string
	Stamp        time.Time
}

type EventWriter interface {
	// LogEvent records a message in the history of a service.
	LogEvent(namespace, service, msg string) error
}

type EventReader interface {
	// AllEvents returns a history for every service. Events must be
	// returned in descending timestamp order.
	AllEvents() ([]Event, error)

	// EventsForService returns the history for a particular
	// service. Events must be returned in descending timestamp order.
	EventsForService(namespace, service string) ([]Event, error)
}

type DB interface {
	LogEvent(inst flux.InstanceID, namespace, service, msg string) error
	AllEvents(inst flux.InstanceID) ([]Event, error)
	EventsForService(inst flux.InstanceID, namespace, service string) ([]Event, error)
	io.Closer
}
