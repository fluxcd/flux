package update

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/policy"
	"github.com/fluxcd/flux/pkg/registry/mock"
)

var (
	name  = mustParseName("index.docker.io/weaveworks/helloworld").CanonicalName()
	infos = []image.Info{
		{ID: name.ToRef("v1"), CreatedAt: time.Now().Add(-time.Hour)},
		{ID: name.ToRef("v2"), CreatedAt: time.Now()},
	}
)

func buildImageRepos(t *testing.T) ImageRepos {
	registry := mock.Registry{Images: infos}
	repoMetadata, err := registry.GetImageRepositoryMetadata(name.Name)
	assert.NoError(t, err)
	return ImageRepos{
		imageRepos: imageReposMap{name.Name.CanonicalName(): repoMetadata},
	}
}

func getFilteredAndSortedImagesFromRepos(t *testing.T, imageName string, repos ImageRepos) SortedImageInfos {
	metadata := repos.GetRepositoryMetadata(mustParseName(imageName))
	images, err := FilterAndSortRepositoryMetadata(metadata, policy.PatternAll)
	assert.NoError(t, err)
	return images
}

// TestDecanon checks that we return appropriate image names when
// asked for images. The registry (cache) stores things with canonical
// names (e.g., `index.docker.io/library/alpine`), but we ask
// questions in terms of everyday names (e.g., `alpine`).
func TestDecanon(t *testing.T) {
	imageRepos := buildImageRepos(t)

	images := getFilteredAndSortedImagesFromRepos(t, "weaveworks/helloworld", imageRepos)
	latest, ok := images.Latest()
	if !ok {
		t.Error("did not find latest image")
	} else if latest.ID.Name != mustParseName("weaveworks/helloworld") {
		t.Error("name did not match what was asked")
	}

	images = getFilteredAndSortedImagesFromRepos(t, "index.docker.io/weaveworks/helloworld", imageRepos)
	latest, ok = images.Latest()
	if !ok {
		t.Error("did not find latest image")
	} else if latest.ID.Name != mustParseName("index.docker.io/weaveworks/helloworld") {
		t.Error("name did not match what was asked")
	}

	avail := getFilteredAndSortedImagesFromRepos(t, "weaveworks/helloworld", imageRepos)
	if len(avail) != len(infos) {
		t.Errorf("expected %d available images, got %d", len(infos), len(avail))
	}
	for _, im := range avail {
		if im.ID.Name != mustParseName("weaveworks/helloworld") {
			t.Errorf("got image with name %q", im.ID.String())
		}
	}
}

func TestMetadataInConsistencyTolerance(t *testing.T) {
	imageRepos := buildImageRepos(t)
	metadata := imageRepos.GetRepositoryMetadata(mustParseName("weaveworks/helloworld"))
	images, err := FilterAndSortRepositoryMetadata(metadata, policy.NewPattern("semver:*"))
	assert.NoError(t, err)

	// Let's make the metadata inconsistent by adding a non-semver tag
	metadata.Tags = append(metadata.Tags, "latest")
	// Filtering and sorting should still work
	images2, err := FilterAndSortRepositoryMetadata(metadata, policy.NewPattern("semver:*"))
	assert.NoError(t, err)
	assert.Equal(t, images, images2)

	// However, an inconsistency in a semver tag should make filtering and sorting fail
	metadata.Tags = append(metadata.Tags, "v9")
	_, err = FilterAndSortRepositoryMetadata(metadata, policy.NewPattern("semver:*"))
	assert.Error(t, err)
}

func TestImageInfos_Filter_latest(t *testing.T) {
	latest := image.Info{
		ID: image.Ref{Name: image.Name{Image: "flux"}, Tag: "latest"},
	}
	other := image.Info{
		ID: image.Ref{Name: image.Name{Image: "moon"}, Tag: "v0"},
	}
	images := []image.Info{latest, other}
	assert.Equal(t, []image.Info{latest}, filterImages(images, policy.PatternLatest))
	assert.Equal(t, []image.Info{latest}, filterImages(images, policy.NewPattern("latest")))
	assert.Equal(t, []image.Info{other}, filterImages(images, policy.PatternAll))
	assert.Equal(t, []image.Info{other}, filterImages(images, policy.NewPattern("*")))
}

func TestImageInfos_Filter_semver(t *testing.T) {
	latest := image.Info{ID: image.Ref{Name: image.Name{Image: "flux"}, Tag: "latest"}}
	semver0 := image.Info{ID: image.Ref{Name: image.Name{Image: "moon"}, Tag: "v0.0.1"}}
	semver1 := image.Info{ID: image.Ref{Name: image.Name{Image: "earth"}, Tag: "1.0.0"}}

	filterAndSort := func(images []image.Info, pattern policy.Pattern) SortedImageInfos {
		filtered := FilterImages(images, pattern)
		return SortImages(filtered, pattern)
	}
	images := []image.Info{latest, semver0, semver1}
	assert.Equal(t, SortedImageInfos{semver1, semver0}, filterAndSort(images, policy.NewPattern("semver:*")))
	assert.Equal(t, SortedImageInfos{semver1}, filterAndSort(images, policy.NewPattern("semver:~1")))
}

func TestAvail(t *testing.T) {
	imageRepos := buildImageRepos(t)
	avail := getFilteredAndSortedImagesFromRepos(t, "weaveworks/goodbyeworld", imageRepos)
	if len(avail) > 0 {
		t.Errorf("did not expect available images, but got %#v", avail)
	}
}

func mustParseName(im string) image.Name {
	ref, err := image.ParseRef(im)
	if err != nil {
		panic(err)
	}
	return ref.Name
}
