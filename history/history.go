package history

import (
	"errors"
	"time"
)

type ServiceState string

const (
	StateUnknown    ServiceState = "Unknown"
	StateRest                    = "At rest"
	StateInProgress              = "Release in progress"
)

var (
	ErrNoHistory = errors.New("no history for service")
)

type Event struct {
	Stamp time.Time
	Msg   string
}

type History struct {
	Service string
	State   ServiceState
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
	LogEvent(namespace, service, msg string) error
	// ChangeState changes the current state of a service
	ChangeState(namespace, service string, newState ServiceState) error
}
