package registry

import (
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"

	fluxmetrics "github.com/weaveworks/flux/metrics"
)

type Metrics struct {
	// Latency of Image fetch, that is getting *all* information about
	// an Image
	FetchDuration metrics.Histogram
	// Counts of particular kinds of request
	RequestDuration metrics.Histogram
}

const (
	LabelRepository  = "repository"
	LabelRequestKind = "kind"

	RequestKindTags     = "tags"
	RequestKindMetadata = "metadata"
)

func NewMetrics() Metrics {
	return Metrics{
		FetchDuration: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "registry",
			Name:      "fetch_duration_seconds",
			Help:      "Duration of Image metadata fetches, in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{fluxmetrics.LabelInstanceID, LabelRepository, fluxmetrics.LabelSuccess}),
		RequestDuration: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "registry",
			Name:      "request_duration_seconds",
			Help:      "Duration of HTTP requests made in the course of fetching Image metadata",
		}, []string{fluxmetrics.LabelInstanceID, LabelRepository, LabelRequestKind, fluxmetrics.LabelSuccess}),
	}
}

func (m Metrics) WithInstanceID(instanceID string) Metrics {
	return Metrics{
		FetchDuration:   m.FetchDuration.With(fluxmetrics.LabelInstanceID, string(instanceID)),
		RequestDuration: m.RequestDuration.With(fluxmetrics.LabelInstanceID, string(instanceID)),
	}
}
