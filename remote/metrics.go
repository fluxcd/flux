package remote

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/job"
	fluxmetrics "github.com/weaveworks/flux/metrics"
	"github.com/weaveworks/flux/update"
)

var (
	requestDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "platform",
		Name:      "request_duration_seconds",
		Help:      "Request duration in seconds.",
		Buckets:   stdprometheus.DefBuckets,
	}, []string{fluxmetrics.LabelMethod, fluxmetrics.LabelSuccess})
)

type instrumentedPlatform struct {
	s api.Server
}

func Instrument(s api.Server) api.Server {
	return &instrumentedPlatform{s}
}

func (i *instrumentedPlatform) Ping(ctx context.Context) (err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "Ping",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.s.Ping(ctx)
}

func (i *instrumentedPlatform) Version(ctx context.Context) (v string, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "Version",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.s.Version(ctx)
}

func (i *instrumentedPlatform) Export(ctx context.Context) (config []byte, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "Export",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.s.Export(ctx)
}

func (i *instrumentedPlatform) ListServices(ctx context.Context, namespace string) (_ []flux.ControllerStatus, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "ListServices",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.s.ListServices(ctx, namespace)
}

func (i *instrumentedPlatform) ListImages(ctx context.Context, spec update.ResourceSpec) (_ []flux.ImageStatus, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "ListImages",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.s.ListImages(ctx, spec)
}

func (i *instrumentedPlatform) UpdateManifests(ctx context.Context, spec update.Spec) (_ job.ID, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "UpdateManifests",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.s.UpdateManifests(ctx, spec)
}

func (i *instrumentedPlatform) NotifyChange(ctx context.Context, change api.Change) (err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "NotifyChange",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.s.NotifyChange(ctx, change)
}

func (i *instrumentedPlatform) JobStatus(ctx context.Context, id job.ID) (_ job.Status, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "JobStatus",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.s.JobStatus(ctx, id)
}

func (i *instrumentedPlatform) SyncStatus(ctx context.Context, cursor string) (_ []string, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "SyncStatus",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.s.SyncStatus(ctx, cursor)
}

func (i *instrumentedPlatform) GitRepoConfig(ctx context.Context, regenerate bool) (_ flux.GitConfig, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "GitRepoConfig",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.s.GitRepoConfig(ctx, regenerate)
}
