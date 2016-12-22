package jobs

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"

	"github.com/weaveworks/flux"
)

const (
	LabelMethod  = "method"
	LabelSuccess = "success"
)

type Metrics struct {
	RequestDuration metrics.Histogram
}

func NewMetrics() Metrics {
	return Metrics{
		RequestDuration: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "jobs",
			Name:      "request_duration_seconds",
			Help:      "Request duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{LabelMethod, LabelSuccess}),
	}
}

type instrumentedJobStore struct {
	js JobStore
	m  Metrics
}

func InstrumentedJobStore(js JobStore, m Metrics) JobStore {
	return &instrumentedJobStore{js, m}
}

func (i *instrumentedJobStore) GetJob(inst flux.InstanceID, jobID JobID) (j Job, err error) {
	defer func(begin time.Time) {
		i.m.RequestDuration.With(
			LabelMethod, "GetJob",
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.js.GetJob(inst, jobID)
}

func (i *instrumentedJobStore) PutJob(inst flux.InstanceID, j Job) (jobID JobID, err error) {
	defer func(begin time.Time) {
		i.m.RequestDuration.With(
			LabelMethod, "PutJob",
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.js.PutJob(inst, j)
}

func (i *instrumentedJobStore) PutJobIgnoringDuplicates(inst flux.InstanceID, j Job) (jobID JobID, err error) {
	defer func(begin time.Time) {
		i.m.RequestDuration.With(
			LabelMethod, "PutJobIgnoringDuplicates",
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.js.PutJobIgnoringDuplicates(inst, j)
}

func (i *instrumentedJobStore) UpdateJob(j Job) (err error) {
	defer func(begin time.Time) {
		i.m.RequestDuration.With(
			LabelMethod, "UpdateJob",
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.js.UpdateJob(j)
}

func (i *instrumentedJobStore) Heartbeat(jobID JobID) (err error) {
	defer func(begin time.Time) {
		i.m.RequestDuration.With(
			LabelMethod, "UpdateJob",
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.js.Heartbeat(jobID)
}

func (i *instrumentedJobStore) NextJob(queues []string) (j Job, err error) {
	defer func(begin time.Time) {
		i.m.RequestDuration.With(
			LabelMethod, "NextJob",
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.js.NextJob(queues)
}

func (i *instrumentedJobStore) GC() (err error) {
	defer func(begin time.Time) {
		i.m.RequestDuration.With(
			LabelMethod, "GC",
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.js.GC()
}
