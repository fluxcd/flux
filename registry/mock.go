package registry

import (
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
)

type mockClientAdapter struct {
	imgs []flux.Image
	err  error
}

type mockRemote struct {
	img  flux.Image
	tags []string
	err  error
}

type ManifestFunc func(repository Repository, tag string) (flux.Image, error)
type TagsFunc func(repository Repository) ([]string, error)
type mockDockerClient struct {
	manifest ManifestFunc
	tags     TagsFunc
}

func NewMockClient(manifest ManifestFunc, tags TagsFunc) Client {
	return &mockDockerClient{
		manifest: manifest,
		tags:     tags,
	}
}

func (m *mockDockerClient) Manifest(repository Repository, tag string) (flux.Image, error) {
	return m.manifest(repository, tag)
}

func (m *mockDockerClient) Tags(repository Repository) ([]string, error) {
	return m.tags(repository)
}

func (*mockDockerClient) Cancel() {
	return
}

type mockRemoteFactory struct {
	c   Client
	err error
}

func NewMockClientFactory(c Client, err error) ClientFactory {
	return &mockRemoteFactory{
		c:   c,
		err: err,
	}
}

func (m *mockRemoteFactory) ClientFor(repository string) (Client, error) {
	return m.c, m.err
}

type mockRegistry struct {
	imgs []flux.Image
	err  error
}

func NewMockRegistry(images []flux.Image, err error) Registry {
	return &mockRegistry{
		imgs: images,
		err:  err,
	}
}

func (m *mockRegistry) GetRepository(repository Repository) ([]flux.Image, error) {
	var imgs []flux.Image
	for _, i := range m.imgs {
		// include only if it's the same repository in the same place
		if i.ID.NamespaceImage() == repository.NamespaceImage() {
			imgs = append(imgs, i)
		}
	}
	return imgs, m.err
}

func (m *mockRegistry) GetImage(repository Repository, tag string) (flux.Image, error) {
	if len(m.imgs) > 0 {
		for _, i := range m.imgs {
			if i.ID.String() == repository.ToImage(tag).ID.String() {
				return i, nil
			}
		}
	}
	return flux.Image{}, errors.New("not found")
}
