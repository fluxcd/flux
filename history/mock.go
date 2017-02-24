package history

import (
	"github.com/weaveworks/flux"
)

type mock struct{}

func NewMock() interface {
	EventReader
	EventWriter
} {
	return mock{}
}

func (m mock) AllEvents() ([]flux.Event, error) {
	return nil, nil
}

func (m mock) EventsForService(_ flux.ServiceID) ([]flux.Event, error) {
	return nil, nil
}

func (m mock) GetEvent(_ flux.EventID) (flux.Event, error) {
	return flux.Event{}, nil
}

func (m mock) LogEvent(_ flux.Event) error {
	return nil
}
