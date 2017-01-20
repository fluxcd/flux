package instance

import (
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/registry"
	"testing"
)

var (
	exampleImage   = "index.docker.io/owner/repo:tag"
	parsedImage, _ = registry.ParseImage(exampleImage, nil)
	testRegistry   = registry.NewMockRegistry([]registry.Image{
		parsedImage,
	}, nil)
)

func TestInstance_ImageExists(t *testing.T) {
	i := Instance{
		registry: testRegistry,
	}
	testImageExists(t, i, exampleImage, true)
	testImageExists(t, i, "owner/repo", false) // False because latest doesn't exist in repo above
	testImageExists(t, i, "repo", false)       // False because latest doesn't exist in repo above
	testImageExists(t, i, "owner/repo:tag", true)
	testImageExists(t, i, "repo:tag", false) // False because the namespaces is owner, not library
	testImageExists(t, i, "owner:tag", false)
}

func testImageExists(t *testing.T, i Instance, image string, expected bool) {
	b, err := i.imageExists(flux.ParseImageID(image))
	if err != nil {
		t.Fatalf("%v: error when requesting image %q", err.Error(), image)
	}
	if b != expected {
		t.Fatalf("For image %q, expected exist = %q but got %q", image, expected, b)
	}
}

func TestInstance_ErrWhenBlank(t *testing.T) {
	i := Instance{
		registry: testRegistry,
	}
	_, err := i.imageExists(flux.ParseImageID(""))
	if err == nil {
		t.Fatal("Was expecting error")
	}
}
