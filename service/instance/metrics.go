package instance

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	fluxmetrics "github.com/weaveworks/flux/metrics"
	"github.com/weaveworks/flux/service"
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

func (i *instrumentedDB) UpdateConfig(inst service.InstanceID, update UpdateFunc) (err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			LabelMethod, "UpdateConfig",
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.UpdateConfig(inst, update)
}

func (i *instrumentedDB) GetConfig(inst service.InstanceID) (c Config, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			LabelMethod, "GetConfig",
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.GetConfig(inst)
}

func (i *instrumentedDB) UpdateGitUrl(inst service.InstanceID, url string) (err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			LabelMethod, "UpdateGitUrl",
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.UpdateGitUrl(inst, url)
}

func (i *instrumentedDB) GetGitUrl(inst service.InstanceID) (u string, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			LabelMethod, "GetGitUrl",
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.db.GetGitUrl(inst)
}
