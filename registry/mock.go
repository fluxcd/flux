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

func NewMockClientAdapter(imgs []flux.ImageDescription, err error) Client {
	return &mockClientAdapter{
		imgs: imgs,
		err:  err,
	}
}

func (m *mockClientAdapter) GetRepository(repository string) (res []flux.ImageDescription, err error) {
	return m.imgs, m.err
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

func (r *mockRemote) Tags(img Image) ([]string, error) {
	return r.tags, r.err
}

func (r *mockRemote) Manifest(img Image) (Image, error) {
	if img.Tag == "error" {
		return Image{}, errors.New("Mock is set to error when tag == error")
	}
	return r.img, r.err
}

func (r *mockRemote) Cancel() {
}

type mockDockerClient struct {
	manifest schema1.SignedManifest
	tags     []string
	err      error
}

func NewMockDockerClient(manifest schema1.SignedManifest, tags []string, err error) dockerRegistryInterface {
	return &mockDockerClient{
		manifest: manifest,
		tags:     tags,
		err:      err,
	}
}

func (m *mockDockerClient) Manifest(repository, reference string) (*schema1.SignedManifest, error) {
	return &m.manifest, m.err
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

func (m *mockRemoteFactory) Create(id Image) (Remote, error) {
	return m.r, m.err
}
