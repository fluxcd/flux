package release

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
)

type serviceQuery interface {
	String() string
	SelectServices(definedServices []flux.ServiceID) ([]flux.ServiceID, error)
}

// Build the selector query to filter to only the service definitions we care
// about.
func serviceSelector(inst *instance.Instance, includeSpecs []flux.ServiceSpec, exclude []flux.ServiceID) (serviceQuery, error) {
	excludeSet := flux.ServiceIDSet{}
	excludeSet.Add(exclude)

	locked, err := lockedServices(inst)
	if err != nil {
		return nil, err
	}
	excludeSet.Add(locked)

	var include flux.ServiceIDSet
	for _, spec := range includeSpecs {
		if spec == flux.ServiceSpecAll {
			// If one of the specs is '<all>' we can ignore the rest.
			return allServicesExcept(excludeSet), nil
		}
		serviceID, err := flux.ParseServiceID(string(spec))
		if err != nil {
			return nil, errors.Wrapf(err, "parsing service ID from params %q", spec)
		}
		include.Add([]flux.ServiceID{serviceID})
	}
	return exactlyTheseServices(include.Without(excludeSet)), nil
}

type funcServiceQuery struct {
	text string
	f    func(definedServices []flux.ServiceID) ([]flux.ServiceID, error)
}

func (f funcServiceQuery) String() string {
	return f.text
}

func (f funcServiceQuery) SelectServices(definedServices []flux.ServiceID) ([]flux.ServiceID, error) {
	return f.f(definedServices)
}

func exactlyTheseServices(include flux.ServiceIDSet) serviceQuery {
	idText := make([]string, len(include))
	for id := range include {
		idText = append(idText, string(id))
	}
	return funcServiceQuery{
		text: strings.Join(idText, ", "),
		f: func(definedServices []flux.ServiceID) ([]flux.ServiceID, error) {
			// Intersect the defined services, with the requested ones.
			var ids []flux.ServiceID
			for id := range flux.ServiceIDs(definedServices).Intersection(include) {
				ids = append(ids, id)
			}
			return ids, nil
		},
	}
}

func allServicesExcept(exclude flux.ServiceIDSet) serviceQuery {
	text := "all services"
	if len(exclude) > 0 {
		idText := make([]string, len(exclude))
		for id := range exclude {
			idText = append(idText, string(id))
		}
		text += fmt.Sprintf(" (except: %s)", strings.Join(idText, ", "))
	}
	return funcServiceQuery{
		text: text,
		f: func(definedServices []flux.ServiceID) ([]flux.ServiceID, error) {
			// Take all defined services, which are not in the excluded ids set
			var ids []flux.ServiceID
			for _, id := range flux.ServiceIDs(definedServices).Without(exclude) {
				ids = append(ids, id)
			}
			return ids, nil
		},
	}
}

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
