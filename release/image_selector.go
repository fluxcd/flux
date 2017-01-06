package release

import (
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/platform"
)

type imageSelector func(*instance.Instance, []platform.Service) (instance.ImageMap, error)

func allLatestImages(h *instance.Instance, services []platform.Service) (instance.ImageMap, error) {
	return h.CollectAvailableImages(services)
}

func exactlyTheseImages(images []flux.ImageID) imageSelector {
	return func(h *instance.Instance, _ []platform.Service) (instance.ImageMap, error) {
		return h.ExactImages(images)
	}
}
