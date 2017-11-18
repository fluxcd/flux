package registry

import (
	"time"

	"github.com/weaveworks/flux/image"
)

const (
	requestTimeout = 10 * time.Second
)

type Registry interface {
	GetRepository(image.Name) ([]image.Info, error)
	GetImage(image.Ref) (image.Info, error)
}
