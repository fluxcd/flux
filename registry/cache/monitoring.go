package cache

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"

	fluxmetrics "github.com/weaveworks/flux/metrics"
)

var (
	memcacheRequestDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "memcache",
		Name:      "request_duration_seconds",
		Help:      "Duration of memcache requests, in seconds.",
		Buckets:   stdprometheus.DefBuckets,
	}, []string{fluxmetrics.LabelMethod, fluxmetrics.LabelSuccess})
)

type instrumentedMemcacheClient struct {
	c Client
}

func InstrumentMemcacheClient(c Client) Client {
	return &instrumentedMemcacheClient{
		c: c,
	}
}

func (i *instrumentedMemcacheClient) GetKey(k Key) (_ []byte, err error) {
	defer func(begin time.Time) {
		memcacheRequestDuration.With(
			fluxmetrics.LabelMethod, "GetKey",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.GetKey(k)
}

func (i *instrumentedMemcacheClient) SetKey(k Key, v []byte) (err error) {
	defer func(begin time.Time) {
		memcacheRequestDuration.With(
			fluxmetrics.LabelMethod, "SetKey",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.SetKey(k, v)
}

func (i *instrumentedMemcacheClient) Stop() {
	defer func(begin time.Time) {
		memcacheRequestDuration.With(
			fluxmetrics.LabelMethod, "Stop",
			fluxmetrics.LabelSuccess, "true",
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	i.c.Stop()
}
