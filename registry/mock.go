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
	img  Image
	tags []string
	err  error
}

func NewMockRemote(img Image, tags []string, err error) Remote {
	return &mockRemote{
		img:  img,
		tags: tags,
		err:  err,
	}
}

func (r *mockRemote) Tags(repository Repository) ([]string, error) {
	return r.tags, r.err
}

func (r *mockRemote) Manifest(repository Repository, tag string) (Image, error) {
	if tag == "error" {
		return Image{}, errors.New("Mock is set to error when tag == error")
	}
	return r.img, r.err
}

func (r *mockRemote) Cancel() {
}

type mockDockerClient struct {
	manifest []schema1.History
	tags     []string
	err      error
}

func NewMockDockerClient(manifest []schema1.History, tags []string, err error) dockerRegistryInterface {
	return &mockDockerClient{
		manifest: manifest,
		tags:     tags,
		err:      err,
	}
}

func (m *mockDockerClient) Manifest(repository, reference string) ([]schema1.History, error) {
	return m.manifest, m.err
}

func (m *mockDockerClient) Tags(repository string) ([]string, error) {
	return m.tags, m.err
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
	imgs []Image
	err  error
}

func NewMockRegistry(images []Image, err error) Registry {
	return &mockRegistry{
		imgs: images,
		err:  err,
	}
}

func (m *mockRegistry) GetRepository(repository Repository) ([]Image, error) {
	return m.imgs, m.err
}

func (m *mockRegistry) GetImage(repository Repository, tag string) (Image, error) {
	if len(m.imgs) > 0 {
		for _, i := range m.imgs {
			if i.String() == repository.ToImage(tag).String() {
				return i, nil
			}
		}
	}
	return Image{}, errors.New("not found")
}
