package history

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/service"
)

const (
	LabelMethod  = "method"
	LabelSuccess = "success"
)

var (
	requestDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "history",
		Name:      "request_duration_seconds",
		Help:      "Request duration in seconds.",
		Buckets:   stdprometheus.DefBuckets,
	}, []string{LabelMethod, LabelSuccess})
)

type instrumentedDB struct {
	db DB
}

func InstrumentedDB(db DB) DB {
	return &instrumentedDB{db}
}

func (i *instrumentedDB) LogEvent(inst service.InstanceID, e Event) (err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			LabelMethod, "LogEvent",
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.LogEvent(inst, e)
}

func (i *instrumentedDB) AllEvents(inst service.InstanceID, before time.Time, limit int64, after time.Time) (e []Event, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			LabelMethod, "AllEvents",
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.AllEvents(inst, before, limit, after)
}

func (i *instrumentedDB) EventsForService(inst service.InstanceID, s flux.ServiceID, before time.Time, limit int64, after time.Time) (e []Event, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			LabelMethod, "EventsForService",
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.EventsForService(inst, s, before, limit, after)
}

func (i *instrumentedDB) GetEvent(id EventID) (e Event, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			LabelMethod, "GetEvent",
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.GetEvent(id)
}

func (i *instrumentedDB) Close() (err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			LabelMethod, "Close",
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.Close()
}
