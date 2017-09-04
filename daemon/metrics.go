package daemon

import (
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"

	fluxmetrics "github.com/weaveworks/flux/metrics"
)

var (
	syncDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "daemon",
		Name:      "sync_duration_seconds",
		Help:      "Duration of git-to-cluster syncs, in seconds.",
		Buckets:   stdprometheus.ExponentialBuckets(0.2, 3, 8), // top bucket ~= 21 minutes
	}, []string{fluxmetrics.LabelSuccess})
)
