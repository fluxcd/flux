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

func (m mock) AllEvents(_ time.Time, _ int64) ([]Event, error) {
	return nil, nil
}

func (m mock) EventsForService(_ flux.ServiceID, _ time.Time, _ int64) ([]Event, error) {
	return nil, nil
}

func (m mock) GetEvent(_ EventID) (Event, error) {
	return Event{}, nil
}

func (m mock) LogEvent(_ Event) error {
	return nil
}
