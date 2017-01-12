package instance

import (
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/registry"
	"testing"
	"time"
)

var (
	fixedTime    = time.Unix(1000000000, 0)
	exampleImage = "owner/repo:tag"
	testRegistry = NewMockRegistry([]flux.ImageDescription{
		{
			ID:        flux.ParseImageID(exampleImage),
			CreatedAt: &fixedTime,
		},
	}, nil)
)

func TestSomething(t *testing.T) {
	i := Instance{
		registry: testRegistry,
	}
	testImageExists(t, i, exampleImage, true)
	testImageExists(t, i, "owner/repo", false)
	testImageExists(t, i, "owner:tag", false)
	testImageExists(t, i, "", false)
}

func testImageExists(t *testing.T, i Instance, image string, expected bool) {
	b, err := i.imageExists(flux.ParseImageID(image))
	if err != nil {
		t.Fatal(err.Error())
	}
	if b != expected {
		t.Fatalf("Expected exist = %q but got %q", expected, b)
	}
}

type mockRegistry struct {
	descriptions []flux.ImageDescription
	err          error
}

func NewMockRegistry(descriptions []flux.ImageDescription, err error) registry.Client {
	return &mockRegistry{
		descriptions: descriptions,
		err:          err,
	}
}

func (r *mockRegistry) GetRepository(repository string) ([]flux.ImageDescription, error) {
	return r.descriptions, r.err
}
