package instance

import (
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/history"
)

type EventReadWriter struct {
	inst flux.InstanceID
	db   history.DB
}

func (rw EventReadWriter) LogEvent(e flux.Event) error {
	return rw.db.LogEvent(rw.inst, e)
}

func (rw EventReadWriter) AllEvents() ([]flux.Event, error) {
	return rw.db.AllEvents(rw.inst)
}

func (rw EventReadWriter) EventsForService(service flux.ServiceID) ([]flux.Event, error) {
	return rw.db.EventsForService(rw.inst, service)
}

func (rw EventReadWriter) GetEvent(id flux.EventID) (flux.Event, error) {
	return rw.db.GetEvent(id)
}
