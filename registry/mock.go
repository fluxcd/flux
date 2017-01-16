package registry

import "github.com/weaveworks/flux"

type mockRegistry struct {
	descriptions []flux.ImageDescription
	err          error
}

func NewMockRegistry(descriptions []flux.ImageDescription, err error) Client {
	return &mockRegistry{
		descriptions: descriptions,
		err:          err,
	}
}

func (r *mockRegistry) GetRepository(repository string) ([]flux.ImageDescription, error) {
	return r.descriptions, r.err
}
