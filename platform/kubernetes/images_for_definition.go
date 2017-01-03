package kubernetes

import (
	"fmt"

	"github.com/weaveworks/flux"
)

// ImagesForDefinition returns a list of images in use for this service definition.
func ImagesForDefinition(definition []byte) ([]flux.ImageID, error) {
	return nil, fmt.Errorf("TODO: Implement kubernetes.ImagesForDefinition")
}
