package update

import (
	"testing"
	"time"

	"github.com/weaveworks/flux/image"
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

	filteredImages := m.GetRepoImages(mustParseName("weaveworks/helloworld")).Filter("*")
	latest, ok := filteredImages.Latest()
	if !ok {
		t.Error("did not find latest image")
	} else if latest.ID.Name != mustParseName("weaveworks/helloworld") {
		t.Error("name did not match what was asked")
	}

	filteredImages = m.GetRepoImages(mustParseName("index.docker.io/weaveworks/helloworld")).Filter("*")
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
