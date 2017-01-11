package release

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/platform"
)

type ServiceSelector interface {
	String() string
	SelectServices(*instance.Instance) ([]platform.Service, error)
}

func ServiceSelectorForSpecs(inst *instance.Instance, includeSpecs []flux.ServiceSpec, exclude []flux.ServiceID) (ServiceSelector, error) {
	excludeSet := flux.ServiceIDSet{}
	excludeSet.Add(exclude)

	locked, err := lockedServices(inst)
	if err != nil {
		return nil, err
	}
	excludeSet.Add(locked)

	include := flux.ServiceIDSet{}
	for _, spec := range includeSpecs {
		if spec == flux.ServiceSpecAll {
			// If one of the specs is '<all>' we can ignore the rest.
			return AllServicesExcept(excludeSet), nil
		}
		serviceID, err := flux.ParseServiceID(string(spec))
		if err != nil {
			return nil, errors.Wrapf(err, "parsing service ID from params %q", spec)
		}
		include.Add([]flux.ServiceID{serviceID})
	}
	return ExactlyTheseServices(include.Without(excludeSet)), nil
}

type funcServiceQuery struct {
	text string
	f    func(inst *instance.Instance) ([]platform.Service, error)
}

func (f funcServiceQuery) String() string {
	return f.text
}

func (f funcServiceQuery) SelectServices(inst *instance.Instance) ([]platform.Service, error) {
	return f.f(inst)
}

func ExactlyTheseServices(include flux.ServiceIDSet) ServiceSelector {
	var (
		idText  []string
		idSlice []flux.ServiceID
	)
	text := "no services"
	if len(include) > 0 {
		for id := range include {
			idText = append(idText, string(id))
			idSlice = append(idSlice, id)
		}
		text = strings.Join(idText, ", ")
	}
	return funcServiceQuery{
		text: text,
		f: func(h *instance.Instance) ([]platform.Service, error) {
			return h.GetServices(idSlice)
		},
	}
}

func AllServicesExcept(exclude flux.ServiceIDSet) ServiceSelector {
	text := "all services"
	if len(exclude) > 0 {
		var idText []string
		for id := range exclude {
			idText = append(idText, string(id))
		}
		text += fmt.Sprintf(" (except: %s)", strings.Join(idText, ", "))
	}
	return funcServiceQuery{
		text: text,
		f: func(h *instance.Instance) ([]platform.Service, error) {
			return h.GetAllServicesExcept("", exclude)
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
