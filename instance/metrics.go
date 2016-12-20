package instance

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
	LabelSuccess    = "success"
)

type Metrics struct {
	UpdateConfigDuration metrics.Histogram
	GetConfigDuration    metrics.Histogram
	AllDuration          metrics.Histogram
}

func NewMetrics() Metrics {
	return Metrics{
		UpdateConfigDuration: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "instance",
			Name:      "update_config_duration_seconds",
			Help:      "UpdateConfig method duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{LabelInstanceID, LabelSuccess}),
		GetConfigDuration: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "instance",
			Name:      "get_config_duration_seconds",
			Help:      "GetConfig method duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{LabelInstanceID, LabelSuccess}),
		AllDuration: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "instance",
			Name:      "all_duration_seconds",
			Help:      "All method duration in seconds.",
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

func (i *instrumentedDB) UpdateConfig(inst flux.InstanceID, update UpdateFunc) (err error) {
	defer func(begin time.Time) {
		i.m.UpdateConfigDuration.With(
			LabelInstanceID, string(inst),
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.UpdateConfig(inst, update)
}

func (i *instrumentedDB) GetConfig(inst flux.InstanceID) (c Config, err error) {
	defer func(begin time.Time) {
		i.m.GetConfigDuration.With(
			LabelInstanceID, string(inst),
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.GetConfig(inst)
}

func (i *instrumentedDB) All() (c []NamedConfig, err error) {
	defer func(begin time.Time) {
		i.m.AllDuration.With(
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.All()
}
