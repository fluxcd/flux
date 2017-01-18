// Monitoring middlewares for registry interfaces
package registry

import (
	"strconv"
	"time"

	"fmt"
	fluxmetrics "github.com/weaveworks/flux/metrics"
)

type InstrumentedRegistry func(Registry) Registry

type registryMonitoringMiddleware struct {
	next    Registry
	metrics Metrics
}

func NewInstrumentedRegistry(metrics Metrics) InstrumentedRegistry {
	return func(next Registry) Registry {
		return &registryMonitoringMiddleware{
			next:    next,
			metrics: metrics,
		}
	}
}

func (m *registryMonitoringMiddleware) GetRepository(repository Repository) (res []Image, err error) {
	start := time.Now()
	res, err = m.next.GetRepository(repository)
	m.metrics.FetchDuration.With(
		LabelRepository, repository.String(),
		fluxmetrics.LabelSuccess, strconv.FormatBool(err == nil),
	).Observe(time.Since(start).Seconds())
	return
}

func (m *registryMonitoringMiddleware) GetImage(repository Repository, tag string) (res Image, err error) {
	start := time.Now()
	res, err = m.next.GetImage(repository, tag)
	m.metrics.FetchDuration.With(
		LabelRepository, fmt.Sprintf("%s:%s", repository.String(), tag),
		fluxmetrics.LabelSuccess, strconv.FormatBool(err == nil),
	).Observe(time.Since(start).Seconds())
	return
}

type InstrumentedRemote func(Remote) Remote

type remoteMonitoringMiddleware struct {
	next    Remote
	metrics Metrics
}

func NewInstrumentedRemote(metrics Metrics) InstrumentedRemote {
	return func(next Remote) Remote {
		return &remoteMonitoringMiddleware{
			next:    next,
			metrics: metrics,
		}
	}
}

func (m *remoteMonitoringMiddleware) Manifest(repository Repository, tag string) (res Image, err error) {
	start := time.Now()
	res, err = m.next.Manifest(repository, tag)
	m.metrics.RequestDuration.With(
		LabelRepository, fmt.Sprintf("%s:%s", repository.String(), tag),
		LabelRequestKind, RequestKindMetadata,
		fluxmetrics.LabelSuccess, strconv.FormatBool(err == nil),
	).Observe(time.Since(start).Seconds())
	return
}

func (m *remoteMonitoringMiddleware) Tags(repository Repository) (res []string, err error) {
	start := time.Now()
	res, err = m.next.Tags(repository)
	m.metrics.RequestDuration.With(
		LabelRepository, repository.String(),
		LabelRequestKind, RequestKindTags,
		fluxmetrics.LabelSuccess, strconv.FormatBool(err == nil),
	).Observe(time.Since(start).Seconds())
	return
}

func (m *remoteMonitoringMiddleware) Cancel() {
	m.next.Cancel()
}
