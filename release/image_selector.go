package release

import (
	"strings"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
)

type imageSelector interface {
	String() string
	SelectImages(*instance.Instance, map[flux.ServiceID][]flux.ImageID) (instance.ImageMap, error)
}

func imageSelectorForSpec(spec flux.ImageSpec) imageSelector {
	switch spec {
	case flux.ImageSpecLatest:
		return allLatestImages
	case flux.ImageSpecNone:
		return latestConfig
	default:
		return exactlyTheseImages([]flux.ImageID{
			flux.ParseImageID(string(spec)),
		})
	}
}

type funcImageSelector struct {
	text string
	f    func(*instance.Instance, map[flux.ServiceID][]flux.ImageID) (instance.ImageMap, error)
}

func (f funcImageSelector) String() string {
	return f.text
}

func (f funcImageSelector) SelectImages(inst *instance.Instance, serviceImages map[flux.ServiceID][]flux.ImageID) (instance.ImageMap, error) {
	return f.f(inst, serviceImages)
}

var (
	allLatestImages = funcImageSelector{
		text: "latest images",
		f: func(h *instance.Instance, serviceImages map[flux.ServiceID][]flux.ImageID) (instance.ImageMap, error) {
			// Collect and unique the images so we can do each once.
			uniqueRepos := map[string]struct{}{}
			for _, images := range serviceImages {
				for _, image := range images {
					uniqueRepos[image.Repository()] = struct{}{}
				}
			}
			repos := make([]string, len(uniqueRepos))
			for repo := range uniqueRepos {
				repos = append(repos, repo)
			}
			// Go look in the registry for new images
			return h.CollectAvailableImages(repos)
		},
	}
	latestConfig = funcImageSelector{
		text: "latest config",
		f: func(h *instance.Instance, serviceImages map[flux.ServiceID][]flux.ImageID) (instance.ImageMap, error) {
			// TODO: Nothing to do here.
			return instance.ImageMap{}, nil
		},
	}
)

func exactlyTheseImages(images []flux.ImageID) imageSelector {
	imageText := make([]string, len(images))
	for _, image := range images {
		imageText = append(imageText, string(image))
	}
	return funcImageSelector{
		text: strings.Join(imageText, ", "),
		f: func(h *instance.Instance, _ map[flux.ServiceID][]flux.ImageID) (instance.ImageMap, error) {
			return h.ExactImages(images)
		},
	}
}
