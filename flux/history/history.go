package history

import (
	"io"
	"time"
)

// Store is an event reader and writer.
// Think of it like an audit history.
type Store interface {
	EventWriter
	EventReader
	io.Closer
}

// TODO(pb): these interfaces need a refactor, e.g.
//  WriteEvent(e Event) error
//  ReadEvents(s flux.ServiceSpec, n int) ([]history.Event, error)

// EventWriter writes events to storage.
type EventWriter interface {
	LogEvent(namespace, service, msg string) error
}

// EventReader reads events from storage.
type EventReader interface {
	AllEvents(namespace string) ([]Event, error)
	EventsForService(namespace, service string) ([]Event, error)
}

// Event is something that happened and should be persisted.
// It is always tied to a specific service. But that could change.
// TODO(pb): use ServiceID instead of Service (string)
type Event struct {
	Service, Msg string
	Stamp        time.Time
}
