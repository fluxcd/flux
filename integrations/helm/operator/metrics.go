package operator

import (
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

var (
	releaseQueueLength = prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
		Namespace: "flux",
		Subsystem: "helm_operator",
		Name:      "release_queue_length_count",
		Help:      "Count of releases waiting in the queue to be processed.",
	}, []string{})
)
