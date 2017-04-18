package registry

import (
	"errors"
	"strings"
	"time"

	"github.com/weaveworks/fluxy/flux"
)

// Registry collects behaviors required from image registries.
type Registry interface {
	// Repository yields a repository matching the given name, if any exists.
	// The repo string may be of various forms, in which case omitted elements
	// take assumed defaults.
	//
	//   helloworld             -> index.docker.io/library/helloworld
	//   foo/helloworld         -> index.docker.io/foo/helloworld
	//   quay.io/foo/helloworld -> quay.io/foo/helloworld
	//
	Repository(repo string) (Repository, error)
}

// These package-level errors should be self-explanatory.
var (
	ErrNoValidImage = errors.New("no valid image available in repository")
)

// Repository is a collection of images with the same registry and name (e.g.
// "quay.io:5000/weaveworks/helloworld") but not the same tag (e.g.
// "quay.io:5000/weaveworks/helloworld:v0.1").
type Repository struct {
	Name   string // "quay.io:5000/weaveworks/helloworld"
	Images []Image
}

// LatestImage returns the latest releasable image from the repository.
// A releasable image is one that is not tagged "latest".
// Images must be kept in newest-first order.
func (r Repository) LatestImage() (Image, error) {
	for _, image := range r.Images {
		_, _, tag := image.ID.Components()
		if strings.EqualFold(tag, "latest") {
			continue
		}
		return image, nil
	}
	return Image{}, ErrNoValidImage
}

// Image represents a specific container image available in a repository.
type Image struct {
	ID        flux.ImageID
	CreatedAt time.Time // always UTC
}
