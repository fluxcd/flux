package release

import (
	"strings"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/platform"
)

type ImageSelector interface {
	String() string
	SelectImages(*instance.Instance, []platform.Service) (instance.ImageMap, error)
}

func ImageSelectorForSpec(spec flux.ImageSpec) ImageSelector {
	switch spec {
	case flux.ImageSpecLatest:
		return AllLatestImages
	case flux.ImageSpecNone:
		return LatestConfig
	default:
		id, _ := flux.ParseImageID(string(spec))
		return ExactlyTheseImages([]flux.ImageID{id})
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
	AllLatestImages = funcImageSelector{
		text: "latest images",
		f: func(h *instance.Instance, services []platform.Service) (instance.ImageMap, error) {
			return h.CollectAvailableImages(services)
		},
	}
	LatestConfig = funcImageSelector{
		text: "latest config",
		f: func(h *instance.Instance, services []platform.Service) (instance.ImageMap, error) {
			// TODO: Nothing to do here.
			return instance.ImageMap{}, nil
		},
	}
)

func ExactlyTheseImages(images []flux.ImageID) ImageSelector {
	var imageText []string
	for _, image := range images {
		imageText = append(imageText, image.String())
	}
	return funcImageSelector{
		text: strings.Join(imageText, ", "),
		f: func(h *instance.Instance, _ []platform.Service) (instance.ImageMap, error) {
			return h.ExactImages(images)
		},
	}
}
