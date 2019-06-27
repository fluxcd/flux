package release

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

const (
	LabelAction      = "action"
	LabelDryRun      = "dry_run"
	LabelSuccess     = "success"
	LabelNamespace   = "namespace"
	LabelReleaseName = "release_name"
)

var (
	durationBuckets = []float64{1, 5, 10, 30, 60, 120, 180, 300}
	releaseDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "helm_operator",
		Name:      "release_duration_seconds",
		Help:      "Release duration in seconds.",
		Buckets:   durationBuckets,
	}, []string{LabelAction, LabelDryRun, LabelSuccess, LabelNamespace, LabelReleaseName})
)

func ObserveRelease(start time.Time, action Action, dryRun, success bool, namespace, releaseName string) {
	releaseDuration.With(
		LabelAction, string(action),
		LabelDryRun, fmt.Sprint(dryRun),
		LabelSuccess, fmt.Sprint(success),
		LabelNamespace, namespace,
		LabelReleaseName, releaseName,
	).Observe(time.Since(start).Seconds())
}
