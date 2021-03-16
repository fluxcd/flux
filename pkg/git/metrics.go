package git

import (
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

const (
	MetricRepoReady   = 1
	MetricRepoUnready = 0
)

var (
	metricGitReady = prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
		Namespace: "flux",
		Subsystem: "git",
		Name:      "ready",
		Help:      "Status of the git repository.",
	}, []string{})
)
