package kubernetes

import (
	"fmt"

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
		// Split up the list items, and iterate over them
		for _, item := range obj.Items {
			ids, err := ImagesForDefinition(item.bytes)
			if err != nil {
				return nil, errors.Wrap(err, "parsing resource definition")
			}
			imageIDs = append(imageIDs, ids...)
		}
	case "Deployment", "ReplicationController":
		// Find the images for each container
		for _, container := range obj.Spec.Template.Spec.Containers {
			imageIDs = append(imageIDs, container.Image)
		}
	default:
		// Ignoring this
		// TODO: Log something here?
	}
	return imageIDs, nil

	definitionStr := string(definition)

	// ${SED} -nr "s/^(\s*)image: $(escape "${image}")://p" "${filename}"
	imageRE := multilineRE(
		`      containers:.*`,
		`(?:      .*\n)*(?:  ){3,4}- name:\s*"?([\w-]+)"?(?:\s.*)?`,
		`(?:  ){4,5}image:\s*"?(`+newImage.Repository()+`:[\w][\w.-]{0,127})"?(\s.*)?`,
	)
	// tag part of regexp from
	// https://github.com/docker/distribution/blob/master/reference/regexp.go#L36

	matches = imageRE.FindStringSubmatch(def)
	if matches == nil || len(matches) < 3 {
		return nil, fmt.Errorf("Could not find image name")
	}
	containerName := matches[1]
	oldImage := flux.ParseImageID(matches[2])
	fmt.Fprintf(trace, "Found container %q using image %v in fragment:\n\n%s\n\n", containerName, oldImage, matches[0])

	return nil, fmt.Errorf("TODO: Implement kubernetes.ImagesForDefinition")
}
