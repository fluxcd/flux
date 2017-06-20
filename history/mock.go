package history

import (
	"database/sql"
	"sync"
	"time"

	"github.com/weaveworks/flux"
)

type Mock struct {
	events []Event
	sync.RWMutex
}

func NewMock() EventReadWriter {
	return &Mock{}
}

func (m *Mock) AllEvents(_ time.Time, _ int64, _ time.Time) ([]Event, error) {
	m.RLock()
	defer m.RUnlock()
	return m.events, nil
}

func (m *Mock) EventsForService(serviceID flux.ServiceID, _ time.Time, _ int64, _ time.Time) ([]Event, error) {
	m.RLock()
	defer m.RUnlock()
	var found []Event
	for _, e := range m.events {
		set := flux.ServiceIDSet{}
		set.Add(e.ServiceIDs)
		if set.Contains(serviceID) {
			found = append(found, e)
		}
	}
	return found, nil
}

func (m *Mock) GetEvent(id EventID) (Event, error) {
	m.RLock()
	defer m.RUnlock()
	for _, e := range m.events {
		if e.ID == id {
			return e, nil
		}
	}
	return Event{}, sql.ErrNoRows
}

func (m *Mock) LogEvent(e Event) error {
	m.Lock()
	defer m.Unlock()
	m.events = append(m.events, e)
	return nil
}
