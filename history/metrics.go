package history

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"

	"github.com/weaveworks/flux"
)

const (
	LabelMethod  = "method"
	LabelSuccess = "success"
)

type Metrics struct {
	RequestDuration metrics.Histogram
}

func NewMetrics() Metrics {
	return Metrics{
		RequestDuration: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "history",
			Name:      "request_duration_seconds",
			Help:      "Request duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{LabelMethod, LabelSuccess}),
	}
}

type instrumentedDB struct {
	db DB
	m  Metrics
}

func InstrumentedDB(db DB, m Metrics) DB {
	return &instrumentedDB{db, m}
}

func (i *instrumentedDB) LogEvent(inst flux.InstanceID, namespace, service, msg string) (err error) {
	defer func(begin time.Time) {
		i.m.RequestDuration.With(
			LabelMethod, "LogEvent",
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.LogEvent(inst, namespace, service, msg)
}

func (i *instrumentedDB) AllEvents(inst flux.InstanceID) (e []Event, err error) {
	defer func(begin time.Time) {
		i.m.RequestDuration.With(
			LabelMethod, "AllEvents",
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.AllEvents(inst)
}

func (i *instrumentedDB) EventsForService(inst flux.InstanceID, namespace, service string) (e []Event, err error) {
	defer func(begin time.Time) {
		i.m.RequestDuration.With(
			LabelMethod, "EventsForService",
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.EventsForService(inst, namespace, service)
}

func (i *instrumentedDB) Close() (err error) {
	defer func(begin time.Time) {
		i.m.CloseDuration.With(
			LabelMethod, "Close",
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.Close()
}
