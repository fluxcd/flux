package history

import "time"

type Event struct {
	Stamp time.Time
	Msg   string
}

type History struct {
	Service string
	Events  []Event
}

type DB interface {
	// AllEvents returns a history for every service in the given
	// namespace
	AllEvents(namespace string) (map[string]History, error)

	// EventsForService returns the history for a particular
	// (namespaced) service
	EventsForService(namespace, service string) (History, error)

	// LogEvent records a message in the history of a service
	LogEvent(namespace, service, msg string)
}
