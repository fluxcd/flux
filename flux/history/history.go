package history

import (
	"io"
	"time"

	"github.com/weaveworks/fluxy/flux"
)

// Store is an event reader and writer.
// Think of it like an audit history.
type Store interface {
	EventWriter
	EventReader
	io.Closer
}

// EventWriter writes events to storage.
type EventWriter interface {
	WriteEvent(Event) error
}

// EventReader reads events from storage.
type EventReader interface {
	ReadEvents(spec flux.ServiceSpec, n int) ([]Event, error)
}

// Event is something that happened and should be persisted.
// It is always tied to a specific service. But that could change.
type Event struct {
	Timestamp time.Time
	Service   flux.ServiceID
	Message   string
}
