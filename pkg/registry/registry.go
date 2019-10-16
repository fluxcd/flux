package registry

import (
	"errors"

	"github.com/fluxcd/flux/pkg/image"
)

var (
	ErrNoImageData = errors.New("image data not available")
)

// Registry is a store of image metadata.
type Registry interface {
	GetImageRepositoryMetadata(image.Name) (image.RepositoryMetadata, error)
	GetImage(image.Ref) (image.Info, error)
}

// ImageCreds is a record of which images need which credentials,
// which is supplied to us (probably by interrogating the cluster)
type ImageCreds map[image.Name]Credentials
