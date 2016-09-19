package fluxsvc

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"

	"github.com/weaveworks/fluxy/flux"
	"github.com/weaveworks/fluxy/flux/automator"
	"github.com/weaveworks/fluxy/flux/history"
	"github.com/weaveworks/fluxy/flux/orgmap"
	"github.com/weaveworks/fluxy/flux/registry"
	"github.com/weaveworks/fluxy/flux/release"
)

// Server implements the fluxsvc service.
type Server struct {
	mapper      orgmap.Mapper         // ListServices, ListImages (via fluxd)
	imageRepo   *registry.Repository  // ListImages
	automator   automator.Automator   // Automate, Deautomate
	releaser    release.JobReadWriter // Release
	eventReader history.EventReader   // History
	logger      log.Logger
	metrics     Metrics
}

func NewServer(
	mapper orgmap.Mapper,
	imageRepo *registry.Repository,
	automator automator.Automator,
	releaser release.JobReadWriter,
	eventReader history.EventReader,
	logger log.Logger,
	metrics Metrics,
) *Server {
	return &Server{
		mapper:      mapper,
		imageRepo:   imageRepo,
		automator:   automator,
		releaser:    releaser,
		eventReader: eventReader,
		logger:      logger,
		metrics:     metrics,
	}
}

// Metrics are recorded by service methods.
type Metrics struct {
	ListServicesDuration metrics.Histogram // namespace, success
	ListImagesDuration   metrics.Histogram // service_spec, success
	ReleaseDuration      metrics.Histogram // kind, success
	AutomateDuration     metrics.Histogram // success
	DeautomateDuration   metrics.Histogram // success
	HistoryDuration      metrics.Histogram // success
}

func (s *Server) ListServices(orgID string, namespace string) (res []flux.ServiceStatus, err error) {
	defer func(begin time.Time) {
		s.metrics.ListServicesDuration.With(
			"namespace", namespace,
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	return nil, errors.New("not implemented")
}

func (s *Server) ListImages(orgID string, service flux.ServiceSpec) (res []flux.ImageStatus, err error) {
	defer func(begin time.Time) {
		s.metrics.ListImagesDuration.With(
			"service_spec", fmt.Sprint(service),
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	return nil, errors.New("not implemented")
}

func (s *Server) Release(orgID string, service flux.ServiceSpec, image flux.ImageSpec, kind flux.ReleaseKind) (res []flux.ReleaseAction, err error) {
	defer func(begin time.Time) {
		s.metrics.ListImagesDuration.With(
			"kind", fmt.Sprint(kind),
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	return nil, errors.New("not implemented")
}

func (s *Server) Automate(orgID string, service flux.ServiceID) (err error) {
	defer func(begin time.Time) {
		s.metrics.AutomateDuration.With(
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	return errors.New("not implemented")
}

func (s *Server) Deautomate(orgID string, service flux.ServiceID) (err error) {
	defer func(begin time.Time) {
		s.metrics.DeautomateDuration.With(
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	return errors.New("not implemented")
}

func (s *Server) History(orgID string, service flux.ServiceSpec, n int) (res []history.Event, err error) {
	defer func(begin time.Time) {
		s.metrics.HistoryDuration.With(
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	return nil, errors.New("not implemented")
}
