package server

import (
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

var (
	connectedDaemons = prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
		Namespace: "flux",
		Subsystem: "fluxsvc",
		Name:      "connected_daemons_count",
		Help:      "Gauge of the current number of connected daemons",
	}, []string{})
)
