package registry

import (
	"time"

	"github.com/weaveworks/flux"
)

type Repository struct {
	img flux.Image // Internally we use an image to store data
}

func RepositoryFromImage(img flux.Image) Repository {
	return Repository{
		img: img,
	}
}

func ParseRepository(imgStr string) (Repository, error) {
	i, err := flux.ParseImage(imgStr, time.Time{})
	if err != nil {
		return Repository{}, err
	}
	return Repository{
		img: i,
	}, nil
}

func (r Repository) NamespaceImage() string {
	return r.img.ID.NamespaceImage()
}

func (r Repository) Host() string {
	return r.img.ID.Host
}

func (r Repository) String() string {
	return r.img.ID.HostNamespaceImage()
}

func (r Repository) ToImage(tag string) flux.Image {
	newImage := r.img
	newImage.ID.Tag = tag
	return newImage
}
