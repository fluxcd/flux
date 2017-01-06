package release

import (
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/platform"
)

type serviceSelector func(*instance.Instance) ([]platform.Service, error)

func exactlyTheseServices(include []flux.ServiceID) serviceSelector {
	return func(h *instance.Instance) ([]platform.Service, error) {
		return h.GetServices(include)
	}
}

func allServicesExcept(exclude flux.ServiceIDSet) serviceSelector {
	return func(h *instance.Instance) ([]platform.Service, error) {
		return h.GetAllServicesExcept("", exclude)
	}
}
