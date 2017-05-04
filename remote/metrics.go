package remote

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/job"
	fluxmetrics "github.com/weaveworks/flux/metrics"
	"github.com/weaveworks/flux/update"
)

var (
	requestDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "platform",
		Name:      "request_duration_seconds",
		Help:      "Request duration in seconds.",
		Buckets:   stdprometheus.DefBuckets,
	}, []string{fluxmetrics.LabelMethod, fluxmetrics.LabelSuccess})
)

type instrumentedPlatform struct {
	p Platform
}

func Instrument(p Platform) Platform {
	return &instrumentedPlatform{p}
}

func (i *instrumentedPlatform) Ping() (err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "Ping",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.p.Ping()
}

func (i *instrumentedPlatform) Version() (v string, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "Version",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.p.Version()
}

func (i *instrumentedPlatform) Export() (config []byte, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "Export",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.p.Export()
}

func (i *instrumentedPlatform) ListServices(namespace string) (_ []flux.ServiceStatus, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "ListServices",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.p.ListServices(namespace)
}

func (i *instrumentedPlatform) ListImages(spec update.ServiceSpec) (_ []flux.ImageStatus, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "ListImages",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.p.ListImages(spec)
}

func (i *instrumentedPlatform) UpdateManifests(spec update.Spec) (_ job.ID, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "UpdateManifests",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.p.UpdateManifests(spec)
}

func (i *instrumentedPlatform) SyncNotify() (err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "SyncNotify",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.p.SyncNotify()
}

func (i *instrumentedPlatform) JobStatus(id job.ID) (_ job.Status, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "JobStatus",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.p.JobStatus(id)
}

func (i *instrumentedPlatform) SyncStatus(cursor string) (_ []string, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "SyncStatus",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.p.SyncStatus(cursor)
}

// BusMetrics has metrics for messages buses.
type BusMetrics struct {
	KickCount metrics.Counter
}

var (
	BusMetricsImpl = BusMetrics{
		KickCount: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: "flux",
			Subsystem: "bus",
			Name:      "kick_total",
			Help:      "Count of bus subscriptions kicked off by a newer subscription.",
		}, []string{}),
	}
)

func (m BusMetrics) IncrKicks(inst flux.InstanceID) {
	m.KickCount.Add(1)
}
