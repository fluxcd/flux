package platform

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"

	"github.com/weaveworks/flux"
	fluxmetrics "github.com/weaveworks/flux/metrics"
)

type Metrics struct {
	RequestDuration metrics.Histogram
}

func NewMetrics() Metrics {
	return Metrics{
		RequestDuration: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "platform",
			Name:      "request_duration_seconds",
			Help:      "Request duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{fluxmetrics.LabelMethod, fluxmetrics.LabelSuccess}),
	}
}

type instrumentedPlatform struct {
	p Platform
	m Metrics
}

func Instrument(p Platform, m Metrics) Platform {
	return &instrumentedPlatform{p, m}
}

func (i *instrumentedPlatform) AllServices(maybeNamespace string, ignored flux.ServiceIDSet) (svcs []Service, err error) {
	defer func(begin time.Time) {
		i.m.RequestDuration.With(
			fluxmetrics.LabelMethod, "AllServices",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.p.AllServices(maybeNamespace, ignored)
}

func (i *instrumentedPlatform) SomeServices(ids []flux.ServiceID) (svcs []Service, err error) {
	defer func(begin time.Time) {
		i.m.RequestDuration.With(
			fluxmetrics.LabelMethod, "SomeServices",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.p.SomeServices(ids)
}

func (i *instrumentedPlatform) Apply(defs []ServiceDefinition) (err error) {
	defer func(begin time.Time) {
		i.m.RequestDuration.With(
			fluxmetrics.LabelMethod, "Apply",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.p.Apply(defs)
}

func (i *instrumentedPlatform) Ping() (err error) {
	defer func(begin time.Time) {
		i.m.RequestDuration.With(
			fluxmetrics.LabelMethod, "Ping",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.p.Ping()
}

// BusMetrics has metrics for messages buses.
type BusMetrics struct {
	KickCount metrics.Counter
}

func NewBusMetrics() BusMetrics {
	return BusMetrics{
		KickCount: prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: "flux",
			Subsystem: "bus",
			Name:      "kick_total",
			Help:      "Count of bus subscriptions kicked off by a newer subscription.",
		}, []string{fluxmetrics.LabelInstanceID}),
	}
}

func (m BusMetrics) IncrKicks(inst flux.InstanceID) {
	m.KickCount.With(fluxmetrics.LabelInstanceID, string(inst)).Add(1)
}
