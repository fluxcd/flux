package kubernetes

import (
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

var (
	ignored = prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
		Namespace: "flux",
		Subsystem: "daemon",
		Name:      "ignored",
		Help:      "Whether a given resource is currently being ignored.",
	}, []string{"resource"})
)
