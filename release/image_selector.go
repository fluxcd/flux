package releaser

import (
	"strings"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/platform"
)

type imageSelector interface {
	String() string
	SelectImages(*instance.Instance, []platform.Service) (instance.ImageMap, error)
}

func imageSelectorForSpec(spec flux.ImageSpec) imageSelector {
	switch params.ImageSpec {
	case flux.ImageSpecLatest:
		return allLatestImages
	case flux.ImageSpecNone:
		return latestConfig
	default:
		return exactlyTheseImages([]flux.ImageID{
			flux.ParseImageID(string(params.ImageSpec)),
		})
	}
}

type funcImageSelector struct {
	text string
	f    func(*instance.Instance, []platform.Service) (instance.ImageMap, error)
}

func (f funcImageSelector) String() string {
	return f.text
}

func (f funcImageSelector) SelectImages(inst *instance.Instance, services []platform.Service) (instance.ImageMap, error) {
	return f.f(inst, services)
}

var (
	allLatestImages = funcImageSelector{
		text: "latest images",
		f: func(h *instance.Instance, services []platform.Service) (instance.ImageMap, error) {
			return h.CollectAvailableImages(services)
		},
	}
	latestConfig = funcImageSelector{
		text: "latest config",
		f: func(h *instance.Instance, services []platform.Service) (instance.ImageMap, error) {
			return instance.ImageMap{}, nil
		},
	}
)

func exactlyTheseImages(images []flux.ImageID) imageSelector {
	return funcImageSelector{
		text: strings.Join(images, ", "),
		f: func(h *instance.Instance, _ []platform.Service) (instance.ImageMap, error) {
			return h.ExactImages(images)
		},
	}
}
