package server

import (
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	fluxmetrics "github.com/weaveworks/flux/metrics"
)

var (
	statusDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "fluxsvc",
		Name:      "status_duration_seconds",
		Help:      "Status method duration in seconds.",
		Buckets:   stdprometheus.DefBuckets,
	}, []string{fluxmetrics.LabelSuccess})
	listServicesDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "fluxsvc",
		Name:      "list_services_duration_seconds",
		Help:      "ListServices method duration in seconds.",
		Buckets:   stdprometheus.DefBuckets,
	}, []string{fluxmetrics.LabelSuccess})
	listImagesDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "fluxsvc",
		Name:      "list_images_duration_seconds",
		Help:      "ListImages method duration in seconds.",
		Buckets:   stdprometheus.DefBuckets,
	}, []string{"service_spec_all", fluxmetrics.LabelSuccess})
	historyDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "fluxsvc",
		Name:      "history_duration_seconds",
		Help:      "History method duration in seconds.",
		Buckets:   stdprometheus.DefBuckets,
	}, []string{"service_spec_all", fluxmetrics.LabelSuccess})
	registerDaemonDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "fluxsvc",
		Name:      "register_daemon_duration_seconds",
		Help:      "RegisterDaemon method duration in seconds.",
		Buckets:   stdprometheus.DefBuckets,
	}, []string{fluxmetrics.LabelSuccess})
	connectedDaemons = prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
		Namespace: "flux",
		Subsystem: "fluxsvc",
		Name:      "connected_daemons_count",
		Help:      "Gauge of the current number of connected daemons",
	}, []string{})
)
