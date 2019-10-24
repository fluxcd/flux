package daemon

import (
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"

	fluxmetrics "github.com/fluxcd/flux/pkg/metrics"
)

var (
	// For us, syncs (of about 100 resources) take about thirty
	// seconds to a minute. Most short-lived (<1s) syncs will be failures.
	syncDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "daemon",
		Name:      "sync_duration_seconds",
		Help:      "Duration of git-to-cluster synchronisation, in seconds.",
		Buckets:   []float64{0.5, 5, 10, 20, 30, 40, 50, 60, 75, 90, 120, 240},
	}, []string{fluxmetrics.LabelSuccess})

	// For most jobs, the majority of the time will be spent pushing
	// changes (git objects and refs) upstream.
	jobDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "daemon",
		Name:      "job_duration_seconds",
		Help:      "Duration of job execution, in seconds.",
		Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 15, 20, 30, 45, 60, 120},
	}, []string{fluxmetrics.LabelSuccess})

	// Same buckets as above (on the rough and ready assumption that
	// jobs will wait for some small multiple of job execution times)
	queueDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "daemon",
		Name:      "queue_duration_seconds",
		Help:      "Duration of time spent in the job queue before execution, in seconds.",
		Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 15, 20, 30, 45, 60, 120},
	}, []string{})

	queueLength = prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
		Namespace: "flux",
		Subsystem: "daemon",
		Name:      "queue_length_count",
		Help:      "Count of jobs waiting in the queue to be run.",
	}, []string{})

	syncManifestsMetric = prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
		Namespace: "flux",
		Subsystem: "daemon",
		Name:      "sync_manifests",
		Help:      "Number of synchronized manifests",
	}, []string{fluxmetrics.LabelSuccess})
)
