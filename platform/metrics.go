package platform

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"

	"github.com/weaveworks/flux"
)

type Metrics struct {
	AllServicesDuration  metrics.Histogram
	SomeServicesDuration metrics.Histogram
	RegradeDuration      metrics.Histogram
	PingDuration         metrics.Histogram
}

func NewMetrics() Metrics {
	return Metrics{
		AllServicesDuration: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "platform",
			Name:      "all_services_duration_seconds",
			Help:      "AllServices method duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{"namespace", "success"}),
		SomeServicesDuration: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "platform",
			Name:      "some_services_duration_seconds",
			Help:      "SomeServices method duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{"success"}),
		RegradeDuration: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "platform",
			Name:      "regrade_duration_seconds",
			Help:      "Regrade method duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{"success"}),
		PingDuration: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "platform",
			Name:      "ping_duration_seconds",
			Help:      "Ping method duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{"success"}),
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
		i.m.AllServicesDuration.With(
			"namespace", maybeNamespace,
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.p.AllServices(maybeNamespace, ignored)
}

func (i *instrumentedPlatform) SomeServices(ids []flux.ServiceID) (svcs []Service, err error) {
	defer func(begin time.Time) {
		i.m.SomeServicesDuration.With(
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.p.SomeServices(ids)
}

func (i *instrumentedPlatform) Regrade(spec []RegradeSpec) (err error) {
	defer func(begin time.Time) {
		i.m.RegradeDuration.With(
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.p.Regrade(spec)
}

func (i *instrumentedPlatform) Ping() (err error) {
	defer func(begin time.Time) {
		i.m.PingDuration.With(
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.p.Ping()
}
