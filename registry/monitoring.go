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

type InstrumentedRegistry Registry

type instrumentedRegistry struct {
	next Registry
}

func NewInstrumentedRegistry(next Registry) InstrumentedRegistry {
	return &instrumentedRegistry{
		next: next,
	}
}

func (m *instrumentedRegistry) GetRepository(id flux.ImageID) (res []flux.Image, err error) {
	start := time.Now()
	res, err = m.next.GetRepository(id)
	registryDuration.With(
		fluxmetrics.LabelSuccess, strconv.FormatBool(err == nil),
	).Observe(time.Since(start).Seconds())
	return
}

func (m *instrumentedRegistry) GetImage(id flux.ImageID) (res flux.Image, err error) {
	start := time.Now()
	res, err = m.next.GetImage(id)
	registryDuration.With(
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

func (m *instrumentedClient) Manifest(id flux.ImageID) (res flux.Image, err error) {
	start := time.Now()
	res, err = m.next.Manifest(id)
	remoteDuration.With(
		LabelRequestKind, RequestKindMetadata,
		fluxmetrics.LabelSuccess, strconv.FormatBool(err == nil),
	).Observe(time.Since(start).Seconds())
	return
}

func (m *instrumentedClient) Tags(id flux.ImageID) (res []string, err error) {
	start := time.Now()
	res, err = m.next.Tags(id)
	remoteDuration.With(
		LabelRequestKind, RequestKindTags,
		fluxmetrics.LabelSuccess, strconv.FormatBool(err == nil),
	).Observe(time.Since(start).Seconds())
	return
}

func (m *instrumentedClient) Cancel() {
	m.next.Cancel()
}
