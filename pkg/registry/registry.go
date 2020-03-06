package registry

import (
	"errors"

	"github.com/fluxcd/flux/pkg/image"
)

var (
	ErrNoImageData       = errors.New("image data not available")
	ErrImageScanDisabled = errors.New("cannot perfom operation, image scanning is disabled")
)

// Registry is a store of image metadata.
type Registry interface {
	GetImageRepositoryMetadata(image.Name) (image.RepositoryMetadata, error)
	GetImage(image.Ref) (image.Info, error)
}

// ImageCreds is a record of which images need which credentials,
// which is supplied to us (probably by interrogating the cluster)
type ImageCreds map[image.Name]Credentials

// ImageScanDisabledRegistry is used when image scanning is disabled
type ImageScanDisabledRegistry struct{}

func (i ImageScanDisabledRegistry) GetImageRepositoryMetadata(image.Name) (image.RepositoryMetadata, error) {
	return image.RepositoryMetadata{}, ErrImageScanDisabled
}

func (i ImageScanDisabledRegistry) GetImage(image.Ref) (image.Info, error) {
	return image.Info{}, ErrImageScanDisabled
}
