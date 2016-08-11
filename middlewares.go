package flux

import (
	"time"

	"github.com/go-kit/kit/log"
	"github.com/weaveworks/fluxy/history"
	"github.com/weaveworks/fluxy/platform"
	"github.com/weaveworks/fluxy/registry"
)

// Middleware is a service-domain (business logic) middleware.
type Middleware func(Service) Service

// LoggingMiddleware returns a middleware that logs every request,
// including parameters, result, and duration.
func LoggingMiddleware(logger log.Logger) Middleware {
	return func(next Service) Service {
		return loggingMiddleware{
			next:   next,
			logger: logger,
		}
	}
}

type loggingMiddleware struct {
	next   Service
	logger log.Logger
}

func (mw loggingMiddleware) Images(repository string) (res []registry.Image, err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "Images",
			"repository", repository,
			"images", len(res),
			"err", err,
			"took", time.Since(begin),
		)
	}(time.Now())
	return mw.next.Images(repository)
}

func (mw loggingMiddleware) ServiceImages(namespace, service string) (res []ContainerImages, err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "ServiceImages",
			"namespace", namespace,
			"service", service,
			"containers", len(res),
			"err", err,
			"took", time.Since(begin),
		)
	}(time.Now())
	return mw.next.ServiceImages(namespace, service)
}

func (mw loggingMiddleware) Services(namespace string) (res []platform.Service, err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "Services",
			"namespace", namespace,
			"services", len(res),
			"err", err,
			"took", time.Since(begin),
		)
	}(time.Now())
	return mw.next.Services(namespace)
}

func (mw loggingMiddleware) History(namespace, service string) (hs []history.Event, err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "History",
			"namespace", namespace,
			"service", service,
			"histories", len(hs),
			"err", err,
			"took", time.Since(begin),
		)
	}(time.Now())
	return mw.next.History(namespace, service)
}

func (mw loggingMiddleware) Release(namespace, service, image string, newDef []byte, updatePeriod time.Duration) (err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "Release",
			"namespace", namespace,
			"service", service,
			"image", image,
			"newDefBytes", len(newDef),
			"updatePeriod", updatePeriod.String(),
			"err", err,
			"took", time.Since(begin),
		)
	}(time.Now())
	return mw.next.Release(namespace, service, image, newDef, updatePeriod)
}

func (mw loggingMiddleware) Automate(namespace, service string) (err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "Automate",
			"namespace", namespace,
			"service", service,
			"err", err,
			"took", time.Since(begin),
		)
	}(time.Now())
	return mw.next.Automate(namespace, service)
}

func (mw loggingMiddleware) Deautomate(namespace, service string) (err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "Deautomate",
			"namespace", namespace,
			"service", service,
			"err", err,
			"took", time.Since(begin),
		)
	}(time.Now())
	return mw.next.Deautomate(namespace, service)
}
