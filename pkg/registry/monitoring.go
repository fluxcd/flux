package registry

// Monitoring middlewares for registry interfaces

import (
	"context"
	"strconv"
	"time"

	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"

	"github.com/fluxcd/flux/pkg/image"
	fluxmetrics "github.com/fluxcd/flux/pkg/metrics"
)

const (
	LabelRequestKind    = "kind"
	RequestKindTags     = "tags"
	RequestKindMetadata = "metadata"
)

var (
	registryDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "registry",
		Name:      "fetch_duration_seconds",
		Help:      "Duration of image metadata requests (from cache), in seconds.",
		Buckets:   stdprometheus.DefBuckets,
	}, []string{fluxmetrics.LabelSuccess})
	remoteDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "client",
		Name:      "fetch_duration_seconds",
		Help:      "Duration of remote image metadata requests, in seconds",
	}, []string{LabelRequestKind, fluxmetrics.LabelSuccess})
)

type instrumentedRegistry struct {
	next Registry
}

func NewInstrumentedRegistry(next Registry) Registry {
	return &instrumentedRegistry{
		next: next,
	}
}

func (m *instrumentedRegistry) GetImageRepositoryMetadata(id image.Name) (res image.RepositoryMetadata, err error) {
	start := time.Now()
	res, err = m.next.GetImageRepositoryMetadata(id)
	registryDuration.With(
		fluxmetrics.LabelSuccess, strconv.FormatBool(err == nil),
	).Observe(time.Since(start).Seconds())
	return
}

func (m *instrumentedRegistry) GetImage(id image.Ref) (res image.Info, err error) {
	start := time.Now()
	res, err = m.next.GetImage(id)
	registryDuration.With(
		fluxmetrics.LabelSuccess, strconv.FormatBool(err == nil),
	).Observe(time.Since(start).Seconds())
	return
}

type instrumentedClient struct {
	next Client
}

func NewInstrumentedClient(next Client) Client {
	return &instrumentedClient{
		next: next,
	}
}

func (m *instrumentedClient) Manifest(ctx context.Context, ref string) (res ImageEntry, err error) {
	start := time.Now()
	res, err = m.next.Manifest(ctx, ref)
	remoteDuration.With(
		LabelRequestKind, RequestKindMetadata,
		fluxmetrics.LabelSuccess, strconv.FormatBool(err == nil),
	).Observe(time.Since(start).Seconds())
	return
}

func (m *instrumentedClient) Tags(ctx context.Context) (res []string, err error) {
	start := time.Now()
	res, err = m.next.Tags(ctx)
	remoteDuration.With(
		LabelRequestKind, RequestKindTags,
		fluxmetrics.LabelSuccess, strconv.FormatBool(err == nil),
	).Observe(time.Since(start).Seconds())
	return
}
