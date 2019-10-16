package mock

import (
	"context"

	"github.com/pkg/errors"

	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/registry"
)

type Client struct {
	ManifestFn func(ref string) (registry.ImageEntry, error)
	TagsFn     func() ([]string, error)
}

func (m *Client) Manifest(ctx context.Context, tag string) (registry.ImageEntry, error) {
	return m.ManifestFn(tag)
}

func (m *Client) Tags(context.Context) ([]string, error) {
	return m.TagsFn()
}

var _ registry.Client = &Client{}

type ClientFactory struct {
	Client registry.Client
	Err    error
}

func (m *ClientFactory) ClientFor(repository image.CanonicalName, creds registry.Credentials) (registry.Client, error) {
	return m.Client, m.Err
}

func (_ *ClientFactory) Succeed(_ image.CanonicalName) {
	return
}

var _ registry.ClientFactory = &ClientFactory{}

type Registry struct {
	Images []image.Info
	Err    error
}

func (m *Registry) GetImageRepositoryMetadata(id image.Name) (image.RepositoryMetadata, error) {
	result := image.RepositoryMetadata{
		Images: map[string]image.Info{},
	}
	for _, i := range m.Images {
		// include only if it's the same repository in the same place
		if i.ID.Image == id.Image {
			tag := i.ID.Tag
			result.Tags = append(result.Tags, tag)
			result.Images[tag] = i
		}
	}
	return result, m.Err
}

func (m *Registry) GetImage(id image.Ref) (image.Info, error) {
	for _, i := range m.Images {
		if i.ID.String() == id.String() {
			return i, nil
		}
	}
	return image.Info{}, errors.New("not found")
}

var _ registry.Registry = &Registry{}
