package update

import (
	"fmt"
	"time"

	fluxmetrics "github.com/fluxcd/flux/pkg/metrics"
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
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

func ObserveRelease(start time.Time, success bool, releaseType ReleaseType, releaseKind ReleaseKind) {
	releaseDuration.With(
		fluxmetrics.LabelSuccess, fmt.Sprint(success),
		fluxmetrics.LabelReleaseType, string(releaseType),
		fluxmetrics.LabelReleaseKind, string(releaseKind),
	).Observe(time.Since(start).Seconds())
}
