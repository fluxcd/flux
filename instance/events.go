package instance

import (
	"github.com/weaveworks/fluxy"
	"github.com/weaveworks/fluxy/history"
)

type EventReadWriter struct {
	inst flux.InstanceID
	db   history.DB
}

func (rw EventReadWriter) LogEvent(namespace, service, msg string) error {
	return rw.db.LogEvent(rw.inst, namespace, service, msg)
}

func (rw EventReadWriter) AllEvents() ([]history.Event, error) {
	return rw.db.AllEvents(rw.inst)
}

func (rw EventReadWriter) EventsForService(namespace, service string) ([]history.Event, error) {
	return rw.db.EventsForService(rw.inst, namespace, service)
}
