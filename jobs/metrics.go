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
	LabelInstanceID = "instance_id"
	LabelNamespace  = "namespace"
	LabelSuccess    = "success"
)

type Metrics struct {
	GetJobDuration                   metrics.Histogram
	PutJobDuration                   metrics.Histogram
	PutJobIgnoringDuplicatesDuration metrics.Histogram
	UpdateJobDuration                metrics.Histogram
	HeartbeatDuration                metrics.Histogram
	NextJobDuration                  metrics.Histogram
	GCDuration                       metrics.Histogram
}

func NewMetrics() Metrics {
	return Metrics{
		GetJobDuration: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "jobs",
			Name:      "get_job_duration_seconds",
			Help:      "GetJob method duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{LabelInstanceID, LabelSuccess}),
		PutJobDuration: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "jobs",
			Name:      "put_job_duration_seconds",
			Help:      "PutJob method duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{LabelInstanceID, LabelSuccess}),
		PutJobIgnoringDuplicatesDuration: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "jobs",
			Name:      "put_job_ignoring_duplicates_duration_seconds",
			Help:      "PutJobIgnoringDuplicates method duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{LabelInstanceID, LabelSuccess}),
		UpdateJobDuration: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "jobs",
			Name:      "update_job_duration_seconds",
			Help:      "UpdateJob method duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{LabelSuccess}),
		HeartbeatDuration: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "jobs",
			Name:      "heartbeat_duration_seconds",
			Help:      "Heartbeat method duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{LabelSuccess}),
		NextJobDuration: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "jobs",
			Name:      "next_job_duration_seconds",
			Help:      "NextJob method duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{LabelSuccess}),
		GCDuration: prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
			Namespace: "flux",
			Subsystem: "jobs",
			Name:      "gc_duration_seconds",
			Help:      "GC method duration in seconds.",
			Buckets:   stdprometheus.DefBuckets,
		}, []string{LabelSuccess}),
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
		i.m.GetJobDuration.With(
			LabelInstanceID, string(inst),
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.js.GetJob(inst, jobID)
}

func (i *instrumentedJobStore) PutJob(inst flux.InstanceID, j Job) (jobID JobID, err error) {
	defer func(begin time.Time) {
		i.m.PutJobDuration.With(
			LabelInstanceID, string(inst),
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.js.PutJob(inst, j)
}

func (i *instrumentedJobStore) PutJobIgnoringDuplicates(inst flux.InstanceID, j Job) (jobID JobID, err error) {
	defer func(begin time.Time) {
		i.m.PutJobIgnoringDuplicatesDuration.With(
			LabelInstanceID, string(inst),
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.js.PutJobIgnoringDuplicates(inst, j)
}

func (i *instrumentedJobStore) UpdateJob(j Job) (err error) {
	defer func(begin time.Time) {
		i.m.UpdateJobDuration.With(
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.js.UpdateJob(j)
}

func (i *instrumentedJobStore) Heartbeat(jobID JobID) (err error) {
	defer func(begin time.Time) {
		i.m.HeartbeatDuration.With(
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.js.Heartbeat(jobID)
}

func (i *instrumentedJobStore) NextJob(queues []string) (j Job, err error) {
	defer func(begin time.Time) {
		i.m.NextJobDuration.With(
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.js.NextJob(queues)
}

func (i *instrumentedJobStore) GC() (err error) {
	defer func(begin time.Time) {
		i.m.GCDuration.With(
			LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.js.GC()
}
