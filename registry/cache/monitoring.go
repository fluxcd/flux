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
	next Client
}

func InstrumentMemcacheClient(c Client) Client {
	return &instrumentedMemcacheClient{
		next: c,
	}
}

func (i *instrumentedMemcacheClient) GetKey(k Key) (_ []byte, err error) {
	defer func(begin time.Time) {
		memcacheRequestDuration.With(
			fluxmetrics.LabelMethod, "GetKey",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.next.GetKey(k)
}

func (i *instrumentedMemcacheClient) GetExpiration(k Key) (_ time.Time, err error) {
	defer func(begin time.Time) {
		memcacheRequestDuration.With(
			fluxmetrics.LabelMethod, "GetExpiration",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.next.GetExpiration(k)
}

func (i *instrumentedMemcacheClient) SetKey(k Key, v []byte) (err error) {
	defer func(begin time.Time) {
		memcacheRequestDuration.With(
			fluxmetrics.LabelMethod, "SetKey",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.next.SetKey(k, v)
}

func (i *instrumentedMemcacheClient) Stop() {
	defer func(begin time.Time) {
		memcacheRequestDuration.With(
			fluxmetrics.LabelMethod, "Stop",
			fluxmetrics.LabelSuccess, "true",
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	i.next.Stop()
}
