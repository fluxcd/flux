package registry

import (
	"github.com/docker/distribution/manifest/schema1"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
)

type mockClientAdapter struct {
	imgs []flux.ImageDescription
	err  error
}

type mockRemote struct {
	img  flux.Image
	tags []string
	err  error
}

func NewMockRemote(img flux.Image, tags []string, err error) Remote {
	return &mockRemote{
		img:  img,
		tags: tags,
		err:  err,
	}
}

func (r *mockRemote) Tags(repository Repository) ([]string, error) {
	return r.tags, r.err
}

func (r *mockRemote) Manifest(repository Repository, tag string) (flux.Image, error) {
	if tag == "error" {
		return flux.Image{}, errors.New("Mock is set to error when tag == error")
	}
	return r.img, r.err
}

func (r *mockRemote) Cancel() {
}

type mockDockerClient struct {
	manifest func(repository, reference string) ([]schema1.History, error)
	tags     func(repository string) ([]string, error)
}

func NewMockDockerClient(manifest func(repository, reference string) ([]schema1.History, error), tags func(repository string) ([]string, error)) dockerRegistryInterface {
	return &mockDockerClient{
		manifest: manifest,
		tags:     tags,
	}
}

func (m *mockDockerClient) Manifest(repository, reference string) ([]schema1.History, error) {
	return m.manifest(repository, reference)
}

func (m *mockDockerClient) Tags(repository string) ([]string, error) {
	return m.tags(repository)
}

type mockRemoteFactory struct {
	r   Remote
	err error
}

func NewMockRemoteFactory(r Remote, err error) RemoteClientFactory {
	return &mockRemoteFactory{
		r:   r,
		err: err,
	}
}

func (m *mockRemoteFactory) CreateFor(repository string) (Remote, error) {
	return m.r, m.err
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
		if i.ImageID.NamespaceImage() == repository.NamespaceImage() {
			imgs = append(imgs, i)
		}
	}
	return imgs, m.err
}

func (m *mockRegistry) GetImage(repository Repository, tag string) (flux.Image, error) {
	if len(m.imgs) > 0 {
		for _, i := range m.imgs {
			if i.String() == repository.ToImage(tag).String() {
				return i, nil
			}
		}
	}
	return flux.Image{}, errors.New("not found")
}
