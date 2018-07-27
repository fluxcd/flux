package update

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/policy"
)

var (
	name  = mustParseName("index.docker.io/weaveworks/helloworld").CanonicalName()
	infos = []image.Info{
		{ID: name.ToRef("v1"), CreatedAt: time.Now().Add(-time.Hour)},
		{ID: name.ToRef("v2"), CreatedAt: time.Now()},
	}
)

// TestDecanon checks that we return appropriate image names when
// asked for images. The registry (cache) stores things with canonical
// names (e.g., `index.docker.io/library/alpine`), but we ask
// questions in terms of everyday names (e.g., `alpine`).
func TestDecanon(t *testing.T) {
	m := ImageRepos{imageReposMap{
		name: infos,
	}}

	filteredImages := m.GetRepoImages(mustParseName("weaveworks/helloworld")).Filter(policy.PatternAll)
	latest, ok := filteredImages.Latest()
	if !ok {
		t.Error("did not find latest image")
	} else if latest.ID.Name != mustParseName("weaveworks/helloworld") {
		t.Error("name did not match what was asked")
	}

	filteredImages = m.GetRepoImages(mustParseName("index.docker.io/weaveworks/helloworld")).Filter(policy.PatternAll)
	latest, ok = filteredImages.Latest()
	if !ok {
		t.Error("did not find latest image")
	} else if latest.ID.Name != mustParseName("index.docker.io/weaveworks/helloworld") {
		t.Error("name did not match what was asked")
	}

	avail := m.GetRepoImages(mustParseName("weaveworks/helloworld"))
	if len(avail) != len(infos) {
		t.Errorf("expected %d available images, got %d", len(infos), len(avail))
	}
	for _, im := range avail {
		if im.ID.Name != mustParseName("weaveworks/helloworld") {
			t.Errorf("got image with name %q", im.ID.String())
		}
	}
}

func TestImageInfos_Filter_latest(t *testing.T) {
	latest := image.Info{
		ID: image.Ref{Name: image.Name{Image: "flux"}, Tag: "latest"},
	}
	other := image.Info{
		ID: image.Ref{Name: image.Name{Image: "moon"}, Tag: "v0"},
	}
	ii := ImageInfos{latest, other}
	assert.Equal(t, ImageInfos{latest}, ii.Filter(policy.PatternLatest))
	assert.Equal(t, ImageInfos{latest}, ii.Filter(policy.NewPattern("latest")))
	assert.Equal(t, ImageInfos{other}, ii.Filter(policy.PatternAll))
	assert.Equal(t, ImageInfos{other}, ii.Filter(policy.NewPattern("*")))
}

func TestImageInfos_Filter_semver(t *testing.T) {
	latest := image.Info{ID: image.Ref{Name: image.Name{Image: "flux"}, Tag: "latest"}}
	semver0 := image.Info{ID: image.Ref{Name: image.Name{Image: "moon"}, Tag: "v0.0.1"}}
	semver1 := image.Info{ID: image.Ref{Name: image.Name{Image: "earth"}, Tag: "1.0.0"}}

	ii := ImageInfos{latest, semver0, semver1}
	assert.Equal(t, ImageInfos{semver1, semver0}, ii.Filter(policy.NewPattern("semver:*")))
	assert.Equal(t, ImageInfos{semver1}, ii.Filter(policy.NewPattern("semver:~1")))
}

func TestAvail(t *testing.T) {
	m := ImageRepos{imageReposMap{name: infos}}
	avail := m.GetRepoImages(mustParseName("weaveworks/goodbyeworld"))
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
