package registry

import (
	"github.com/docker/distribution/manifest/schema1"
)

type mockRegistry struct {
	img  Image
	tags []string
	err  error
}

func NewMockRegistry(img Image, tags []string, err error) Remote {
	return &mockRegistry{
		img:  img,
		tags: tags,
		err:  err,
	}
}

func (r *mockRegistry) Tags(img Image) ([]string, error) {
	return r.tags, r.err
}

func (r *mockRegistry) Manifest(img Image) (Image, error) {
	return r.img, r.err
}

func (r *mockRegistry) Cancel() {
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
