package history

import (
	"sync"
	"time"
)

type namespacedService struct {
	namespace, service string
}

func NewInMemDB() DB {
	return &db{
		histories: make(map[namespacedService]*History),
	}
}

type db struct {
	mtx       sync.Mutex
	histories map[namespacedService]*History
}

func newHistory(service string) *History {
	return &History{
		Service: service,
		Events:  []Event{},
	}
}

func (h *History) add(msg string) error {
	t := time.Now()
	h.Events = append([]Event{Event{Stamp: t, Msg: msg}}, h.Events...)
	return nil
}

func (db *db) ensureHistory(namespace, service string) *History {
	ns := namespacedService{namespace, service}
	if h, found := db.histories[ns]; found {
		return h
	}

	h := newHistory(service)
	db.histories[ns] = h
	return h
}

func (db *db) AllEvents(namespace string) (map[string]History, error) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	hs := map[string]History{}
	for _, h := range db.histories {
		hs[h.Service] = *h
	}
	return hs, nil
}

func (db *db) EventsForService(namespace, service string) (History, error) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	if h, found := db.histories[namespacedService{namespace, service}]; found {
		return *h, nil
	}
	return History{
		Service: service,
	}, nil
}

func (db *db) LogEvent(namespace, service, msg string) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	history := db.ensureHistory(namespace, service)
	history.add(msg)
}
