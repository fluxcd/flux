package cache

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"

	fluxmetrics "github.com/fluxcd/flux/pkg/metrics"
)

var (
	cacheRequestDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "cache",
		Name:      "request_duration_seconds",
		Help:      "Duration of cache requests, in seconds.",
		Buckets:   stdprometheus.DefBuckets,
	}, []string{fluxmetrics.LabelMethod, fluxmetrics.LabelSuccess})
)

type instrumentedClient struct {
	next Client
}

func InstrumentClient(c Client) Client {
	return &instrumentedClient{
		next: c,
	}
}

func (i *instrumentedClient) GetKey(k Keyer) (_ []byte, ex time.Time, err error) {
	defer func(begin time.Time) {
		cacheRequestDuration.With(
			fluxmetrics.LabelMethod, "GetKey",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.next.GetKey(k)
}

func (i *instrumentedClient) SetKey(k Keyer, d time.Time, v []byte) (err error) {
	defer func(begin time.Time) {
		cacheRequestDuration.With(
			fluxmetrics.LabelMethod, "SetKey",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.next.SetKey(k, d, v)
}
