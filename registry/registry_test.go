package registry

import (
	"errors"
	"testing"
	"time"

	"github.com/docker/distribution/manifest/schema1"
	"github.com/go-kit/kit/log"

	fluxerr "github.com/weaveworks/flux/errors"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/registry/cache"
	"github.com/weaveworks/flux/registry/middleware"
)

const testTagStr = "latest"
const testImageStr = "alpine:" + testTagStr
const constTime = "2017-01-13T16:22:58.009923189Z"

var (
	id, _ = image.ParseRef(testImageStr)
	man   = schema1.SignedManifest{
		Manifest: schema1.Manifest{
			History: []schema1.History{
				{
					V1Compatibility: `{"created":"` + constTime + `"}`,
				},
			},
		},
	}
)

var (
	testTags = []string{testTagStr, "anotherTag"}
	mClient  = NewMockClient(
		func(_ image.Ref) (image.Info, error) {
			return image.Info{ID: id, CreatedAt: time.Time{}}, nil
		},
		func(repository image.Name) ([]string, error) {
			return testTags, nil
		},
	)
)

func TestRegistry_GetRepository(t *testing.T) {
	fact := NewMockClientFactory(mClient, nil)
	reg := NewRegistry(fact, log.NewNopLogger(), 512)
	imgs, err := reg.GetRepository(id.Name)
	if err != nil {
		t.Fatal(err)
	}
	// Dev note, the tags will look the same because we are returning the same
	// Image from the mock remote. But they are distinct images.
	if len(testTags) != len(imgs) {
		t.Fatal("Expecting %v images, but got %v", len(testTags), len(imgs))
	}
}

func TestRegistry_GetRepositoryFactoryError(t *testing.T) {
	errFact := NewMockClientFactory(mClient, errors.New(""))
	reg := NewRegistry(errFact, nil, 512)
	_, err := reg.GetRepository(id.Name)
	if err == nil {
		t.Fatal("Expecting error")
	}
}

func TestRegistry_GetRepositoryManifestError(t *testing.T) {
	errClient := NewMockClient(
		func(repository image.Ref) (image.Info, error) {
			return image.Info{}, errors.New("")
		},
		func(repository image.Name) ([]string, error) {
			return testTags, nil
		},
	)
	errFact := NewMockClientFactory(errClient, nil)
	reg := NewRegistry(errFact, log.NewNopLogger(), 512)
	_, err := reg.GetRepository(id.Name)
	if err == nil {
		t.Fatal("Expecting error")
	}
}

// Note: This actually goes off to docker hub to find the Image.
// It will fail if there is not internet connection
func TestRemoteFactory_RawClient(t *testing.T) {
	// No credentials required for public Image
	fact := NewRemoteClientFactory(log.NewNopLogger(), middleware.RateLimiterConfig{
		RPS:   200,
		Burst: 1,
	})

	// Refresh tags first
	var tags []string
	client, err := fact.ClientFor(id.Registry(), Credentials{})
	if err != nil {
		t.Fatal(err)
	}

	tags, err = client.Tags(id.Name)
	if err != nil {
		t.Fatal(err)
	}
	client.Cancel()
	if len(tags) == 0 {
		t.Fatal("Should have some tags")
	}

	client, err = fact.ClientFor(id.Registry(), Credentials{})
	if err != nil {
		t.Fatal(err)
	}
	id.Tag = tags[0]
	newImg, err := client.Manifest(id)
	if err != nil {
		t.Fatal(err)
	}
	if newImg.ID.String() == "" {
		t.Fatal("Should image ")
	}
	if newImg.CreatedAt.IsZero() {
		t.Fatal("CreatedAt time was 0")
	}
	client.Cancel()
}

func TestRemoteFactory_InvalidHost(t *testing.T) {
	fact := NewRemoteClientFactory(log.NewNopLogger(), middleware.RateLimiterConfig{})
	invalidId, err := image.ParseRef("invalid.host/library/alpine:latest")
	if err != nil {
		t.Fatal(err)
	}
	client, err := fact.ClientFor(invalidId.Registry(), Credentials{})
	if err != nil {
		return
	}
	_, err = client.Manifest(invalidId)
	if err == nil {
		t.Fatal("Expected error due to invalid host but got none.")
	}
}

func TestRemote_BetterError(t *testing.T) {
	errClient := NewMockClient(
		func(repository image.Ref) (image.Info, error) {
			return image.Info{}, cache.ErrNotCached
		},
		func(repository image.Name) ([]string, error) {
			return []string{}, cache.ErrNotCached
		},
	)

	fact := NewMockClientFactory(errClient, nil)
	reg := NewRegistry(fact, log.NewNopLogger(), 512)
	_, err := reg.GetRepository(id.Name)
	if err == nil {
		t.Fatal("Should have errored")
	}
	if !fluxerr.IsMissing(err) {
		t.Fatalf("Should not be bespoke error, got %q", err.Error())
	}
	_, err = reg.GetImage(id)
	if err == nil {
		t.Fatal("Should have errored")
	}
	if !fluxerr.IsMissing(err) {
		t.Fatalf("Should not be bespoke error, got %q", err.Error())
	}
}
