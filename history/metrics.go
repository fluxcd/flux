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
	LabelInstanceID = "instance_id"
	LabelNamespace  = "namespace"
	LabelSuccess    = "success"
)

type Metrics struct {
	LogEventDuration         metrics.Histogram
	AllEventsDuration        metrics.Histogram
	EventsForServiceDuration metrics.Histogram
	CloseDuration            metrics.Histogram
}

func NewMetrics() Metrics {
	return Metrics{
		LogEventDuration: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "history",
			Name:      "log_event_duration_seconds",
			Help:      "LogEvent method duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{LabelInstanceID, LabelNamespace, LabelSuccess}),
		AllEventsDuration: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "history",
			Name:      "all_events_duration_seconds",
			Help:      "AllEvents method duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{LabelInstanceID, LabelSuccess}),
		EventsForServiceDuration: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "history",
			Name:      "events_for_service_duration_seconds",
			Help:      "EventsForService method duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{LabelInstanceID, LabelNamespace, LabelSuccess}),
		CloseDuration: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "history",
			Name:      "close_duration_seconds",
			Help:      "Close method duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{LabelSuccess}),
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
		i.m.AllEventsDuration.With(
			LabelInstanceID, string(inst),
			LabelNamespace, namespace,
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.LogEvent(inst, namespace, service, msg)
}

func (i *instrumentedDB) AllEvents(inst flux.InstanceID) (e []Event, err error) {
	defer func(begin time.Time) {
		i.m.AllEventsDuration.With(
			LabelInstanceID, string(inst),
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.AllEvents(inst)
}

func (i *instrumentedDB) EventsForService(inst flux.InstanceID, namespace, service string) (e []Event, err error) {
	defer func(begin time.Time) {
		i.m.EventsForServiceDuration.With(
			LabelInstanceID, string(inst),
			LabelNamespace, namespace,
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.EventsForService(inst, namespace, service)
}

func (i *instrumentedDB) Close() (err error) {
	defer func(begin time.Time) {
		i.m.CloseDuration.With(
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.Close()
}
