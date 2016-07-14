package flux

import (
	"time"

	"github.com/go-kit/kit/endpoint"
	"golang.org/x/net/context"

	"github.com/weaveworks/fluxy/platform"
)

// Endpoints collects all of the individual endpoints (one-to-one with methods)
// that comprise a Flux service. It's meant to be used as a helper struct,
// to collect all endpoints into a single parameter.
type Endpoints struct {
	ServicesEndpoint endpoint.Endpoint
	ReleaseEndpoint  endpoint.Endpoint
}

// MakeServerEndpoints returns an Endpoints struct where each endpoint invokes the
// corresponding method on the provided service. Useful in a server i.e. fluxd.
func MakeServerEndpoints(s Service) Endpoints {
	return Endpoints{
		ServicesEndpoint: MakeServicesEndpoint(s),
		ReleaseEndpoint:  MakeReleaseEndpoint(s),
	}
}

// MakeServicesEndpoint returns an endpoint via the passed service.
// Primarily useful in a server.
func MakeServicesEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(servicesRequest)
		services, err := s.Services(req.Namespace)
		return servicesResponse{Services: services, Err: err}, nil
	}
}

// MakeReleaseEndpoint returns an endpoint via the passed service.
// Primarily useful in a server.
func MakeReleaseEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(releaseRequest)
		err := s.Release(req.Namespace, req.Service, req.NewDef, req.UpdatePeriod)
		return releaseResponse{Err: err}, nil
	}
}

type servicesRequest struct {
	Namespace string
}

type servicesResponse struct {
	Services []platform.Service
	Err      error
}

type releaseRequest struct {
	Namespace    string
	Service      string
	NewDef       []byte
	UpdatePeriod time.Duration
}

type releaseResponse struct {
	Err error
}
