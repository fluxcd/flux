package registry

import (
	"fmt"
	"strings"
)

// ImageParts splits the image name apart, returning the repository, image
// name, and tag if possible
func ImageParts(image string) (repository, name, tag string) {
	parts := strings.SplitN(image, "/", 3)
	if len(parts) == 3 {
		repository = parts[0]
		image = fmt.Sprintf("%s/%s", parts[1], parts[2])
	}
	parts = strings.SplitN(image, ":", 2)
	if len(parts) == 2 {
		tag = parts[1]
	}
	return repository, parts[0], tag
}

// ImageFromParts combines an image name and tag
func ImageFromParts(repository, name, tag string) string {
	s := name
	if repository != "" {
		s = repository + "/" + s
	}
	if tag != "" {
		s = s + ":" + tag
	}
	return s
}
