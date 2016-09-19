package fluxd

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/pkg/errors"

	"github.com/weaveworks/fluxy/flux"
	"github.com/weaveworks/fluxy/flux/platform/kubernetes"
)

// Server implements the fluxd service.
type Server struct {
	cluster *kubernetes.Cluster
	logger  log.Logger
	metrics Metrics
}

func NewServer(
	cluster *kubernetes.Cluster,
	logger log.Logger,
	metrics Metrics,
) *Server {
	return &Server{
		cluster: cluster,
		logger:  logger,
		metrics: metrics,
	}
}

// Metrics are recorded by service methods.
type Metrics struct {
	ListServicesDuration metrics.Histogram // namespace, success
	ListImagesDuration   metrics.Histogram // service_spec, success
	ReleaseDuration      metrics.Histogram // kind, success
}

func (s *Server) ListServices(namespace string) (res []flux.ServiceStatus, err error) {
	defer func(begin time.Time) {
		s.metrics.ListServicesDuration.With(
			"namespace", namespace,
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	return nil, errors.New("not implemented")
}

// TODO(pb): this may need to return something smaller than ImageStatus
// as we don't want to perform any interaction with the image repo.
func (s *Server) ListImages(service flux.ServiceSpec) (res []flux.ImageStatus, err error) {
	defer func(begin time.Time) {
		s.metrics.ListImagesDuration.With(
			"service_spec", fmt.Sprint(service),
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	return nil, errors.New("not implemented")
}

func (s *Server) Release(service flux.ServiceSpec, image flux.ImageSpec, kind flux.ReleaseKind) (res []flux.ReleaseAction, err error) {
	defer func(begin time.Time) {
		s.metrics.ListImagesDuration.With(
			"kind", fmt.Sprint(kind),
			"success", fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	return nil, errors.New("not implemented")
}
