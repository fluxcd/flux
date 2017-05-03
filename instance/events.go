package instance

import (
	"time"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/history"
)

type EventReadWriter struct {
	inst flux.InstanceID
	db   history.DB
}

func (rw EventReadWriter) LogEvent(e history.Event) error {
	return rw.db.LogEvent(rw.inst, e)
}

func (rw EventReadWriter) AllEvents(before time.Time, limit int64) ([]history.Event, error) {
	return rw.db.AllEvents(rw.inst, before, limit)
}

func (rw EventReadWriter) EventsForService(service flux.ServiceID, before time.Time, limit int64) ([]history.Event, error) {
	return rw.db.EventsForService(rw.inst, service, before, limit)
}

func (rw EventReadWriter) GetEvent(id history.EventID) (history.Event, error) {
	return rw.db.GetEvent(id)
}
