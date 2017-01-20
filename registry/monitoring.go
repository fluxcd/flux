// Monitoring middlewares for registry interfaces
package registry

import (
	"strconv"
	"time"

	"fmt"
	fluxmetrics "github.com/weaveworks/flux/metrics"
)

type InstrumentedRegistry Registry

type instrumentedRegistry struct {
	next    Registry
	metrics Metrics
}

func NewInstrumentedRegistry(next Registry, metrics Metrics) InstrumentedRegistry {
	return &instrumentedRegistry{
		next:    next,
		metrics: metrics,
	}
}

func (m *instrumentedRegistry) GetRepository(repository Repository) (res []Image, err error) {
	start := time.Now()
	res, err = m.next.GetRepository(repository)
	m.metrics.FetchDuration.With(
		LabelRepository, repository.String(),
		fluxmetrics.LabelSuccess, strconv.FormatBool(err == nil),
	).Observe(time.Since(start).Seconds())
	return
}

func (m *instrumentedRegistry) GetImage(repository Repository, tag string) (res Image, err error) {
	start := time.Now()
	res, err = m.next.GetImage(repository, tag)
	m.metrics.FetchDuration.With(
		LabelRepository, fmt.Sprintf("%s:%s", repository.String(), tag),
		fluxmetrics.LabelSuccess, strconv.FormatBool(err == nil),
	).Observe(time.Since(start).Seconds())
	return
}

type InstrumentedRemote Remote

type instrumentedRemote struct {
	next    Remote
	metrics Metrics
}

func NewInstrumentedRemote(next Remote, metrics Metrics) Remote {
	return &instrumentedRemote{
		next:    next,
		metrics: metrics,
	}
}

func (m *instrumentedRemote) Manifest(repository Repository, tag string) (res Image, err error) {
	start := time.Now()
	res, err = m.next.Manifest(repository, tag)
	m.metrics.RequestDuration.With(
		LabelRepository, fmt.Sprintf("%s:%s", repository.String(), tag),
		LabelRequestKind, RequestKindMetadata,
		fluxmetrics.LabelSuccess, strconv.FormatBool(err == nil),
	).Observe(time.Since(start).Seconds())
	return
}

func (m *instrumentedRemote) Tags(repository Repository) (res []string, err error) {
	start := time.Now()
	res, err = m.next.Tags(repository)
	m.metrics.RequestDuration.With(
		LabelRepository, repository.String(),
		LabelRequestKind, RequestKindTags,
		fluxmetrics.LabelSuccess, strconv.FormatBool(err == nil),
	).Observe(time.Since(start).Seconds())
	return
}

func (m *instrumentedRemote) Cancel() {
	m.next.Cancel()
}
