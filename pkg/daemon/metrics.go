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

	// syncErrorCount provides a way for observing git-to-cluster
	// synchronisation errors without needing to inspect the pod's logs.
	syncErrorCount = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Namespace: "flux",
		Subsystem: "daemon",
		Name:      "sync_error_count",
		Help:      "Count of git-to-cluster synchronisation errors.",
	}, []string{})

	// lastSyncTimestamp will contain the timestamp at which the git-to-cluster
	// synchronisation was last attempted.
	lastSyncTimestamp = prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
		Namespace: "flux",
		Subsystem: "daemon",
		Name:      "last_sync_timestamp",
		Help:      "The timestamp at which git-to-cluster synchronisation was last attempted.",
	}, []string{})

	// lastSuccessfulSyncTimestamp will contain the timestamp at which the
	// git-to-cluster synchronisation was last attempted successfully.
	lastSuccessfulSyncTimestamp = prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
		Namespace: "flux",
		Subsystem: "daemon",
		Name:      "last_successful_sync_timestamp",
		Help:      "The timestamp at which git-to-cluster synchronisation was last successfully attempted.",
	}, []string{})

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
)
