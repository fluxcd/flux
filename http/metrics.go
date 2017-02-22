package http

import (
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	fluxmetrics "github.com/weaveworks/flux/metrics"
)

var (
	requestDuration = stdprometheus.NewHistogramVec(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Name:      "request_duration_seconds",
		Help:      "Time (in seconds) spent serving HTTP requests.",
		Buckets:   stdprometheus.DefBuckets,
	}, []string{fluxmetrics.LabelMethod, fluxmetrics.LabelRoute, "status_code", "ws"})
)
