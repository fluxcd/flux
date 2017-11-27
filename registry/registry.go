package registry

import (
	"time"

	"github.com/weaveworks/flux/image"
)

const (
	requestTimeout = 10 * time.Second
)

// Registry is a store of image metadata.
type Registry interface {
	GetRepository(image.Name) ([]image.Info, error)
	GetImage(image.Ref) (image.Info, error)
}

// Client is a remote registry client. It is an interface so we can
// wrap it in instrumentation, write fake implementations, and so on.
type Client interface {
	Tags(name image.Name) ([]string, error)
	Manifest(name image.Ref) (image.Info, error)
}

// ImageCreds is a record of which images need which credentials,
// which is supplied to us (probably by interrogating the cluster)
type ImageCreds map[image.Name]Credentials
