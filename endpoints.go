package flux

import (
	"time"

	"github.com/go-kit/kit/endpoint"
	"golang.org/x/net/context"

	"github.com/weaveworks/fluxy/platform"
	"github.com/weaveworks/fluxy/registry"
)

// Endpoints collects all of the individual endpoints (one-to-one with methods)
// that comprise a Flux service. It's meant to be used as a helper struct,
// to collect all endpoints into a single parameter.
type Endpoints struct {
	ImagesEndpoint         endpoint.Endpoint
	ServiceImagesEndpoint  endpoint.Endpoint
	ServicesEndpoint       endpoint.Endpoint
	ReleaseEndpoint        endpoint.Endpoint
	ReleasesStatusEndpoint endpoint.Endpoint
}

// MakeServerEndpoints returns an Endpoints struct where each endpoint invokes the
// corresponding method on the provided service. Useful in a server i.e. fluxd.
func MakeServerEndpoints(s Service) Endpoints {
	return Endpoints{
		ImagesEndpoint:         MakeImagesEndpoint(s),
		ServiceImagesEndpoint:  MakeServiceImagesEndpoint(s),
		ServicesEndpoint:       MakeServicesEndpoint(s),
		ReleaseEndpoint:        MakeReleaseEndpoint(s),
		ReleasesStatusEndpoint: MakeReleasesStatusEndpoint(s),
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

// MakeReleaseEndpoint returns an endpoint via the passed service.
// Primarily useful in a server.
func MakeReleaseEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(releaseRequest)
		err := s.Release(req.Namespace, req.Service, req.NewDef, req.UpdatePeriod)
		return releaseResponse{Err: err}, nil
	}
}

func MakeReleasesStatusEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(releasesStatusRequest)
		statuses, err := s.ReleasesStatus(req.Namespace)
		return releasesStatusResponse{Status: statuses, Err: err}, nil
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

type releaseRequest struct {
	Namespace    string
	Service      string
	NewDef       []byte
	UpdatePeriod time.Duration
}

type releaseResponse struct {
	Err error
}

type releasesStatusRequest struct {
	Namespace string
}

type releasesStatusResponse struct {
	Status []ReleaseStatus
	Err    error
}
