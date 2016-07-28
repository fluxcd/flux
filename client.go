package flux

import (
	"net/url"
	"strings"
	"time"

	httptransport "github.com/go-kit/kit/transport/http"
	"golang.org/x/net/context"

	"github.com/weaveworks/fluxy/history"
	"github.com/weaveworks/fluxy/platform"
	"github.com/weaveworks/fluxy/registry"
)

// NewClient takes an instance string and produces a service that invokes that
// instance. We have only one transport, which is HTTP, so that's what's used.
func NewClient(instance string) (Service, error) {
	if !strings.HasPrefix(instance, "http") {
		instance = "http://" + instance
	}
	tgt, err := url.Parse(instance)
	if err != nil {
		return nil, err
	}
	tgt.Path = ""

	options := []httptransport.ClientOption{}

	// Note that the request encoders modify the request URL, changing the path
	// and method and maybe the body. That's fine: we simply need to provide
	// specific encoders for each endpoint.

	return serviceWrapper{
		ctx: context.Background(),
		endpoints: Endpoints{
			ImagesEndpoint:        httptransport.NewClient("GET", tgt, encodeImagesRequest, decodeImagesResponse, options...).Endpoint(),
			ServiceImagesEndpoint: httptransport.NewClient("GET", tgt, encodeServiceImagesRequest, decodeServiceImagesResponse, options...).Endpoint(),
			ServicesEndpoint:      httptransport.NewClient("GET", tgt, encodeServicesRequest, decodeServicesResponse, options...).Endpoint(),
			HistoryEndpoint:       httptransport.NewClient("GET", tgt, encodeHistoryRequest, decodeHistoryResponse, options...).Endpoint(),
			ReleaseEndpoint:       httptransport.NewClient("POST", tgt, encodeReleaseRequest, decodeReleaseResponse, options...).Endpoint(),
		},
	}, nil
}

// serviceWrapper allows an endpoints struct to be used as a service.
type serviceWrapper struct {
	ctx       context.Context
	endpoints Endpoints
}

func (w serviceWrapper) Images(repository string) ([]registry.Image, error) {
	request := imagesRequest{Repository: repository}
	response, err := w.endpoints.ImagesEndpoint(w.ctx, request)
	if err != nil {
		return nil, err
	}
	resp := response.(imagesResponse)
	return resp.Images, resp.Err
}

func (w serviceWrapper) ServiceImages(namespace, service string) ([]ContainerImages, error) {
	request := serviceImagesRequest{Namespace: namespace, Service: service}
	response, err := w.endpoints.ServiceImagesEndpoint(w.ctx, request)
	if err != nil {
		return nil, err
	}
	resp := response.(serviceImagesResponse)
	return resp.ContainerImages, resp.Err
}

func (w serviceWrapper) Services(namespace string) ([]platform.Service, error) {
	request := servicesRequest{Namespace: namespace}
	response, err := w.endpoints.ServicesEndpoint(w.ctx, request)
	if err != nil {
		return nil, err
	}
	resp := response.(servicesResponse)
	return resp.Services, resp.Err
}

func (w serviceWrapper) History(namespace, service string) (map[string]history.History, error) {
	request := historyRequest{namespace, service}
	response, err := w.endpoints.HistoryEndpoint(w.ctx, request)
	if err != nil {
		return nil, err
	}
	resp := response.(historyResponse)
	return resp.History, resp.Err
}

func (w serviceWrapper) Release(namespace, service string, newDef []byte, updatePeriod time.Duration) error {
	request := releaseRequest{
		Namespace:    namespace,
		Service:      service,
		NewDef:       newDef,
		UpdatePeriod: updatePeriod,
	}
	response, err := w.endpoints.ReleaseEndpoint(w.ctx, request)
	if err != nil {
		return err
	}
	resp := response.(releaseResponse)
	return resp.Err
}
