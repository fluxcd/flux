package history

import (
	"database/sql"
	"sync"
	"time"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/event"
)

type Mock struct {
	events []event.Event
	sync.RWMutex
}

func NewMock() *Mock {
	return &Mock{}
}

func (m *Mock) AllEvents(_ time.Time, _ int64, _ time.Time) ([]event.Event, error) {
	m.RLock()
	defer m.RUnlock()
	return m.events, nil
}

func (m *Mock) EventsForService(serviceID flux.ResourceID, _ time.Time, _ int64, _ time.Time) ([]event.Event, error) {
	m.RLock()
	defer m.RUnlock()
	var found []event.Event
	for _, e := range m.events {
		set := flux.ResourceIDSet{}
		set.Add(e.ServiceIDs)
		if set.Contains(serviceID) {
			found = append(found, e)
		}
	}
	return found, nil
}

func (m *Mock) GetEvent(id event.EventID) (event.Event, error) {
	m.RLock()
	defer m.RUnlock()
	for _, e := range m.events {
		if e.ID == id {
			return e, nil
		}
	}
	return event.Event{}, sql.ErrNoRows
}

func (m *Mock) LogEvent(e event.Event) error {
	m.Lock()
	defer m.Unlock()
	m.events = append(m.events, e)
	return nil
}
