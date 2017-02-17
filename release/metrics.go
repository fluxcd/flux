package release

import (
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	fluxmetrics "github.com/weaveworks/flux/metrics"
)

var (
	releaseDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "fluxsvc",
		Name:      "release_duration_seconds",
		Help:      "Release method duration in seconds.",
		Buckets:   stdprometheus.DefBuckets,
	}, []string{fluxmetrics.LabelReleaseType, fluxmetrics.LabelReleaseKind, fluxmetrics.LabelSuccess})
	stageDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "fluxsvc",
		Name:      "release_stage_duration_seconds",
		Help:      "Duration in seconds of each stage of a release, including dry-runs.",
		Buckets:   stdprometheus.DefBuckets,
	}, []string{fluxmetrics.LabelStage})
)

func NewStageTimer(stage string) *metrics.Timer {
	return metrics.NewTimer(stageDuration.With(fluxmetrics.LabelStage, stage))
}
