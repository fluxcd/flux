package daemon

import (
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"

	fluxmetrics "github.com/weaveworks/flux/metrics"
)

var (
	// For us, syncs (of about 100 resources) take about thirty
	// seconds to a minute. Most short-lived (<1s) syncs will be failures.
	syncDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "daemon",
		Name:      "sync_duration_seconds",
		Help:      "Duration of git-to-cluster syncs, in seconds.",
		Buckets:   []float64{0.5, 5, 10, 20, 30, 40, 50, 60, 75, 90, 120, 240},
	}, []string{fluxmetrics.LabelSuccess})

	// For most jobs, the majority of the time will be spent pushing
	// changes (git objects and refs) upstream.
	jobDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "daemon",
		Name:      "job_duration_seconds",
		Help:      "Duration of jobs, in seconds.",
		Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 15, 20, 30, 45, 60, 120},
	}, []string{fluxmetrics.LabelSuccess})
)
