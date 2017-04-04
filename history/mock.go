package history

import (
	"time"

	"github.com/weaveworks/flux"
)

type mock struct{}

func NewMock() interface {
	EventReader
	EventWriter
} {
	return mock{}
}

func (m mock) AllEvents(_ time.Time, _ int64) ([]flux.Event, error) {
	return nil, nil
}

func (m mock) EventsForService(_ flux.ServiceID, _ time.Time, _ int64) ([]flux.Event, error) {
	return nil, nil
}

func (m mock) GetEvent(_ flux.EventID) (flux.Event, error) {
	return flux.Event{}, nil
}

func (m mock) LogEvent(_ flux.Event) error {
	return nil
}
