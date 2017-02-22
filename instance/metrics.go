package instance

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/flux"
	fluxmetrics "github.com/weaveworks/flux/metrics"
)

const (
	LabelMethod  = "method"
	LabelSuccess = "success"
)

var (
	releaseHelperDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "fluxsvc",
		Name:      "release_helper_duration_seconds",
		Help:      "Duration in seconds of a variety of release helper methods.",
		Buckets:   stdprometheus.DefBuckets,
	}, []string{fluxmetrics.LabelMethod, fluxmetrics.LabelSuccess})
	requestDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "instance",
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

func (i *instrumentedDB) UpdateConfig(inst flux.InstanceID, update UpdateFunc) (err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			LabelMethod, "UpdateConfig",
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.UpdateConfig(inst, update)
}

func (i *instrumentedDB) GetConfig(inst flux.InstanceID) (c Config, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			LabelMethod, "GetConfig",
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.GetConfig(inst)
}

func (i *instrumentedDB) All() (c []NamedConfig, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			LabelMethod, "All",
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.All()
}
