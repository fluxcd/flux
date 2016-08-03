package flux

import (
	"time"

	"github.com/go-kit/kit/endpoint"
	"golang.org/x/net/context"

	"github.com/weaveworks/fluxy/history"
	"github.com/weaveworks/fluxy/platform"
	"github.com/weaveworks/fluxy/registry"
)

// Endpoints collects all of the individual endpoints (one-to-one with methods)
// that comprise a Flux service. It's meant to be used as a helper struct,
// to collect all endpoints into a single parameter.
type Endpoints struct {
	ImagesEndpoint        endpoint.Endpoint
	ServiceImagesEndpoint endpoint.Endpoint
	ServicesEndpoint      endpoint.Endpoint
	ServiceEndpoint       endpoint.Endpoint
	HistoryEndpoint       endpoint.Endpoint
	ReleaseEndpoint       endpoint.Endpoint
	AutomateEndpoint      endpoint.Endpoint
	DeautomateEndpoint    endpoint.Endpoint
}

// MakeServerEndpoints returns an Endpoints struct where each endpoint invokes the
// corresponding method on the provided service. Useful in a server i.e. fluxd.
func MakeServerEndpoints(s Service) Endpoints {
	return Endpoints{
		ImagesEndpoint:        MakeImagesEndpoint(s),
		ServiceImagesEndpoint: MakeServiceImagesEndpoint(s),
		ServicesEndpoint:      MakeServicesEndpoint(s),
		ServiceEndpoint:       MakeServiceEndpoint(s),
		HistoryEndpoint:       MakeHistoryEndpoint(s),
		ReleaseEndpoint:       MakeReleaseEndpoint(s),
		AutomateEndpoint:      MakeAutomateEndpoint(s),
		DeautomateEndpoint:    MakeDeautomateEndpoint(s),
	}
}

// MakeImagesEndpoint returns an endpoint via the passed service.
// Primarily useful in a server.
func MakeImagesEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(imagesRequest)
		images, err := s.Images(req.Repository)
		return imagesResponse{Images: images, Err: err}, nil
	}
}

// MakeServiceImagesEndpoint returns an endpoint via the passed service.
// Primarily useful in a server.
func MakeServiceImagesEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(serviceImagesRequest)
		containers, err := s.ServiceImages(req.Namespace, req.Service)
		return serviceImagesResponse{ContainerImages: containers, Err: err}, nil
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

// MakeServiceEndpoint returns an endpoint via the passed service.
// Primarily useful in a server.
func MakeServiceEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(serviceRequest)
		service, err := s.Service(req.Namespace, req.Service)
		return serviceResponse{Service: service, Err: err}, nil
	}
}

// MakeHistoryEndpoint returns an endpoint via the passed service.
// Primarily useful in a server.
func MakeHistoryEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(historyRequest)
		h, err := s.History(req.Namespace, req.Service)
		return historyResponse{History: h, Err: err}, nil
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

// MakeAutomateEndpoint returns an endpoint via the passed service.
// Primarily useful in a server.
func MakeAutomateEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(automateRequest)
		err := s.Automate(req.Namespace, req.Service)
		return automateResponse{Err: err}, nil
	}
}

// MakeDeautomateEndpoint returns an endpoint via the passed service.
// Primarily useful in a server.
func MakeDeautomateEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(deautomateRequest)
		err := s.Deautomate(req.Namespace, req.Service)
		return deautomateResponse{Err: err}, nil
	}
}

type imagesRequest struct {
	Repository string
}

type imagesResponse struct {
	Images []registry.Image
	Err    error
}

type serviceImagesRequest struct {
	Namespace string
	Service   string
}

type serviceImagesResponse struct {
	ContainerImages []ContainerImages
	Err             error
}

type servicesRequest struct {
	Namespace string
}

type servicesResponse struct {
	Services []platform.Service
	Err      error
}

type serviceRequest struct {
	Namespace string
	Service   string
}

type serviceResponse struct {
	Service platform.Service
	Err     error
}

type historyRequest struct {
	Namespace, Service string
}

type historyResponse struct {
	History map[string]history.History
	Err     error
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

type automateRequest struct {
	Namespace string
	Service   string
}

type automateResponse struct {
	Err error
}

type deautomateRequest struct {
	Namespace string
	Service   string
}

type deautomateResponse struct {
	Err error
}
