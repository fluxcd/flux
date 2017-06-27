package update

import (
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/flux/metrics"
)

var (
	releaseDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "fluxsvc",
		Name:      "release_duration_seconds",
		Help:      "Release method duration in seconds.",
		Buckets:   stdprometheus.DefBuckets,
	}, []string{metrics.LabelReleaseType, metrics.LabelReleaseKind, metrics.LabelSuccess})
)
