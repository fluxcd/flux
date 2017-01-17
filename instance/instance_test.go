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
	testRegistry = registry.NewMockClientAdapter([]flux.ImageDescription{
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
		t.Fatalf("%v: error when requesting image %q", err.Error(), image)
	}
	if b != expected {
		t.Fatalf("Expected exist = %q but got %q", expected, b)
	}
}
