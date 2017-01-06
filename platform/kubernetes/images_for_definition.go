package kubernetes

import (
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	"github.com/weaveworks/flux"
)

// ImagesForDefinition returns a list of images in use for this service definition.
func ImagesForDefinition(definition []byte) ([]flux.ImageID, error) {
	obj, err := definitionObj(definition)
	if err != nil {
		return nil, errors.Wrap(err, "parsing resource definition")
	}
	var imageIDs []flux.ImageID
	switch obj.Kind {
	case "List":
		var l list
		if err := yaml.Unmarshal(obj.bytes, &l); err != nil {
			return nil, errors.Wrap(err, "parsing resource definition")
		}
		// Split up the list items, and iterate over them
		for _, item := range l.Items {
			ids, err := ImagesForDefinition(item)
			if err != nil {
				return nil, errors.Wrap(err, "parsing resource definition")
			}
			imageIDs = append(imageIDs, ids...)
		}
	case "Deployment", "ReplicationController", "DaemonSet":
		var p templateContainer
		if err := yaml.Unmarshal(obj.bytes, &p); err != nil {
			return nil, errors.Wrap(err, "parsing resource definition")
		}
		// Find the images for each container
		for _, container := range p.Spec.Template.Spec.Containers {
			imageIDs = append(imageIDs, flux.ParseImageID(container.Image))

		}
	default:
		// Ignoring this
		// TODO: Log something here?
	}
	return imageIDs, nil
}

type list struct {
	Items [][]byte `yaml:"items"`
}

type templateContainer struct {
	Spec struct {
		Template struct {
			Spec struct {
				Containers []struct {
					Image string `yaml:"image"`
				} `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}
