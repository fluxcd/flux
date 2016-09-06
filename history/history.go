package history

import (
	"io"
	"time"
)

type ServiceState string

const (
	StateUnknown    ServiceState = "Unknown"
	StateRest                    = "At rest"
	StateInProgress              = "Release in progress"
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
	// AllEvents returns a history for every service in the given namespace.
	AllEvents(namespace string) ([]Event, error)

	// EventsForService returns the history for a particular service.
	EventsForService(namespace, service string) ([]Event, error)
}

type DB interface {
	EventWriter
	EventReader
	io.Closer
}
