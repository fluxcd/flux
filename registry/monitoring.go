// Monitoring middlewares for registry interfaces
package registry

import (
	"strconv"
	"time"

	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/flux"
	fluxmetrics "github.com/weaveworks/flux/metrics"
)

const (
	LabelRequestKind    = "kind"
	RequestKindTags     = "tags"
	RequestKindMetadata = "metadata"
)

var (
	fetchDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "registry",
		Name:      "fetch_duration_seconds",
		Help:      "Duration of Image metadata fetches, in seconds.",
		Buckets:   stdprometheus.DefBuckets,
	}, []string{fluxmetrics.LabelSuccess})
	requestDuration = prometheus.NewHistogramFrom(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Subsystem: "registry",
		Name:      "request_duration_seconds",
		Help:      "Duration of HTTP requests made in the course of fetching Image metadata",
	}, []string{LabelRequestKind, fluxmetrics.LabelSuccess})
)

type InstrumentedRegistry Registry

type instrumentedRegistry struct {
	next Registry
}

func NewInstrumentedRegistry(next Registry) InstrumentedRegistry {
	return &instrumentedRegistry{
		next: next,
	}
}

func (m *instrumentedRegistry) GetRepository(repository Repository) (res []flux.Image, err error) {
	start := time.Now()
	res, err = m.next.GetRepository(repository)
	fetchDuration.With(
		fluxmetrics.LabelSuccess, strconv.FormatBool(err == nil),
	).Observe(time.Since(start).Seconds())
	return
}

func (m *instrumentedRegistry) GetImage(repository Repository, tag string) (res flux.Image, err error) {
	start := time.Now()
	res, err = m.next.GetImage(repository, tag)
	fetchDuration.With(
		fluxmetrics.LabelSuccess, strconv.FormatBool(err == nil),
	).Observe(time.Since(start).Seconds())
	return
}

type InstrumentedClient Client

type instrumentedClient struct {
	next Client
}

func NewInstrumentedClient(next Client) Client {
	return &instrumentedClient{
		next: next,
	}
}

func (m *instrumentedClient) Manifest(repository Repository, tag string) (res flux.Image, err error) {
	start := time.Now()
	res, err = m.next.Manifest(repository, tag)
	requestDuration.With(
		LabelRequestKind, RequestKindMetadata,
		fluxmetrics.LabelSuccess, strconv.FormatBool(err == nil),
	).Observe(time.Since(start).Seconds())
	return
}

func (m *instrumentedClient) Tags(repository Repository) (res []string, err error) {
	start := time.Now()
	res, err = m.next.Tags(repository)
	requestDuration.With(
		LabelRequestKind, RequestKindTags,
		fluxmetrics.LabelSuccess, strconv.FormatBool(err == nil),
	).Observe(time.Since(start).Seconds())
	return
}

func (m *instrumentedClient) Cancel() {
	m.next.Cancel()
}
