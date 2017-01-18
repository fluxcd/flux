// Monitoring middlewares for registry interfaces
package registry

import (
	"strconv"
	"time"

	fluxmetrics "github.com/weaveworks/flux/metrics"
)

type RegistryMonitoringMiddleware func(Registry) Registry

type registryMonitoringMiddleware struct {
	next    Registry
	metrics Metrics
}

func NewRegistryMonitoringMiddleware(metrics Metrics) RegistryMonitoringMiddleware {
	return func(next Registry) Registry {
		return &registryMonitoringMiddleware{
			next:    next,
			metrics: metrics,
		}
	}
}

func (m *registryMonitoringMiddleware) GetRepository(img Image) (res []Image, err error) {
	start := time.Now()
	res, err = m.next.GetRepository(img)
	m.metrics.FetchDuration.With(
		LabelRepository, img.HostNamespaceImage(),
		fluxmetrics.LabelSuccess, strconv.FormatBool(err == nil),
	).Observe(time.Since(start).Seconds())
	return
}

func (m *registryMonitoringMiddleware) GetImage(img Image) (res Image, err error) {
	start := time.Now()
	res, err = m.next.GetImage(img)
	m.metrics.FetchDuration.With(
		LabelRepository, img.HostNamespaceImage(),
		fluxmetrics.LabelSuccess, strconv.FormatBool(err == nil),
	).Observe(time.Since(start).Seconds())
	return
}

type RemoteMonitoringMiddleware func(Remote) Remote

type remoteMonitoringMiddleware struct {
	next    Remote
	metrics Metrics
}

func NewRemoteMonitoringMiddleware(metrics Metrics) RemoteMonitoringMiddleware {
	return func(next Remote) Remote {
		return &remoteMonitoringMiddleware{
			next:    next,
			metrics: metrics,
		}
	}
}

func (m *remoteMonitoringMiddleware) Manifest(img Image) (res Image, err error) {
	start := time.Now()
	res, err = m.next.Manifest(img)
	m.metrics.RequestDuration.With(
		LabelRepository, img.HostNamespaceImage(),
		LabelRequestKind, RequestKindMetadata,
		fluxmetrics.LabelSuccess, strconv.FormatBool(err == nil),
	).Observe(time.Since(start).Seconds())
	return
}

func (m *remoteMonitoringMiddleware) Tags(img Image) (res []string, err error) {
	start := time.Now()
	res, err = m.next.Tags(img)
	m.metrics.RequestDuration.With(
		LabelRepository, img.HostNamespaceImage(),
		LabelRequestKind, RequestKindTags,
		fluxmetrics.LabelSuccess, strconv.FormatBool(err == nil),
	).Observe(time.Since(start).Seconds())
	return
}

func (m *remoteMonitoringMiddleware) Cancel() {
	m.next.Cancel()
}
