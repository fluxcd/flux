package release

import (
	"github.com/pkg/errors"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/platform"
)

// Build the selector query to fetch the services from the platform
//
// TODO: Why do we need to fetch them from the platform? Surely they need to
// come from the definition files...
func serviceSelector(inst *instance.Instance, includeSpecs []flux.ServiceSpec, exclude []flux.ServiceID) (serviceQuery, error) {
	excludeSet := flux.ServiceIDSet{}
	excludeSet.Add(exclude)

	locked, err := lockedServices(inst)
	if err != nil {
		return nil, err
	}
	excludeSet.Add(locked)

	var include []flux.ServiceID
	for _, spec := range includeSpecs {
		if spec == flux.ServiceSpecAll {
			// If one of the specs is '<all>' we can ignore the rest.
			return allServicesExcept(excludeSet), nil
		}
		serviceID, err := flux.ParseServiceID(string(includeSpecs))
		if err != nil {
			return nil, errors.Wrapf(err, "parsing service ID from params %q", spec)
		}
		include = append(include, serviceID)
	}
	return exactlyTheseServices(flux.ServiceIDs(include).Without(excludeSet)), nil
}

type serviceQuery func(*instance.Instance) (map[flux.ServiceID]map[string][]byte, error)

func exactlyTheseServices(include []flux.ServiceID) serviceQuery {
	return func(h *instance.Instance) ([]platform.Service, error) {
		return h.GetServices(include)
	}
}

func allServicesExcept(exclude flux.ServiceIDSet) serviceQuery {
	return func(h *instance.Instance) ([]platform.Service, error) {
		return h.GetAllServicesExcept("", exclude)
	}
}

// Get set of all locked services
func lockedServices(inst *instance.Instance) ([]flux.ServiceID, error) {
	config, err := inst.GetConfig()
	if err != nil {
		return nil, err
	}

	ids := []flux.ServiceID{}
	for id, s := range config.Services {
		if s.Locked {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
