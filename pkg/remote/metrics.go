package remote

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"

	"github.com/fluxcd/flux/pkg/api"
	"github.com/fluxcd/flux/pkg/api/v10"
	"github.com/fluxcd/flux/pkg/api/v11"
	"github.com/fluxcd/flux/pkg/api/v6"
	"github.com/fluxcd/flux/pkg/api/v9"
	"github.com/fluxcd/flux/pkg/job"
	fluxmetrics "github.com/fluxcd/flux/pkg/metrics"
	"github.com/fluxcd/flux/pkg/update"
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

var _ api.Server = &instrumentedServer{}

type instrumentedServer struct {
	s api.Server
}

func Instrument(s api.Server) *instrumentedServer {
	return &instrumentedServer{s}
}

func (i *instrumentedServer) Export(ctx context.Context) (config []byte, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "Export",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.s.Export(ctx)
}

func (i *instrumentedServer) ListServices(ctx context.Context, namespace string) (_ []v6.ControllerStatus, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "ListServices",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.s.ListServices(ctx, namespace)
}

func (i *instrumentedServer) ListServicesWithOptions(ctx context.Context, opts v11.ListServicesOptions) (_ []v6.ControllerStatus, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "ListServicesWithOptions",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.s.ListServicesWithOptions(ctx, opts)
}

func (i *instrumentedServer) ListImages(ctx context.Context, spec update.ResourceSpec) (_ []v6.ImageStatus, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "ListImages",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.s.ListImages(ctx, spec)
}

func (i *instrumentedServer) ListImagesWithOptions(ctx context.Context, opts v10.ListImagesOptions) (_ []v6.ImageStatus, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "ListImagesWithOptions",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.s.ListImagesWithOptions(ctx, opts)
}

func (i *instrumentedServer) UpdateManifests(ctx context.Context, spec update.Spec) (_ job.ID, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "UpdateManifests",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.s.UpdateManifests(ctx, spec)
}

func (i *instrumentedServer) JobStatus(ctx context.Context, id job.ID) (_ job.Status, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "JobStatus",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.s.JobStatus(ctx, id)
}

func (i *instrumentedServer) SyncStatus(ctx context.Context, cursor string) (_ []string, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "SyncStatus",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.s.SyncStatus(ctx, cursor)
}

func (i *instrumentedServer) GitRepoConfig(ctx context.Context, regenerate bool) (_ v6.GitConfig, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "GitRepoConfig",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.s.GitRepoConfig(ctx, regenerate)
}

func (i *instrumentedServer) Ping(ctx context.Context) (err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "Ping",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.s.Ping(ctx)
}

func (i *instrumentedServer) Version(ctx context.Context) (v string, err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "Version",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.s.Version(ctx)
}

func (i *instrumentedServer) NotifyChange(ctx context.Context, change v9.Change) (err error) {
	defer func(begin time.Time) {
		requestDuration.With(
			fluxmetrics.LabelMethod, "NotifyChange",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())
	return i.s.NotifyChange(ctx, change)
}
