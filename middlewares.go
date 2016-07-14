package flux

import (
	"time"

	"github.com/go-kit/kit/log"
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

func (mw loggingMiddleware) Release(namespace, service string, newDef []byte, updatePeriod time.Duration) (err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "Release",
			"namespace", namespace,
			"service", service,
			"newDefBytes", len(newDef),
			"updatePeriod", updatePeriod.String(),
			"err", err,
			"took", time.Since(begin),
		)
	}(time.Now())
	return mw.next.Release(namespace, service, newDef, updatePeriod)
}
