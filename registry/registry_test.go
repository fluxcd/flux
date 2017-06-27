package registry

import (
	"fmt"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/docker/distribution/manifest/schema1"
	"github.com/weaveworks/flux"
)

var (
	testTags = []string{testTagStr, "anotherTag"}
	mRemote  = NewMockRemote(img, testTags, nil)
	mClient  = NewMockDockerClient(
		func(repository, reference string) ([]schema1.History, error) {
			return []schema1.History{{`{"test":"json"}`}}, nil
		},
		func(repository string) ([]string, error) {
			return testTags, nil
		},
	)
	testTime, _ = time.Parse(time.RFC3339Nano, constTime)
)

func TestRegistry_GetImage(t *testing.T) {
	newImg, err := mRemote.Manifest(testRepository, img.ID.Tag)
	if err != nil {
		t.Fatal(err)
	}
	if img.ID.String() != newImg.ID.String() {
		t.Fatal("Expected %v, but got %v", img.ID.String(), newImg.ID.String())
	}
}

func TestRegistry_GetRepository(t *testing.T) {
	fact := NewMockClientFactory(mClient, nil)
	reg := NewRegistry(fact, log.NewNopLogger())
	imgs, err := reg.GetRepository(testRepository)
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
	reg := NewRegistry(errFact, nil)
	_, err := reg.GetRepository(testRepository)
	if err == nil {
		t.Fatal("Expecting error")
	}
}

func TestRegistry_GetRepositoryManifestError(t *testing.T) {
	errClient := NewMockDockerClient(
		func(repository, reference string) ([]schema1.History, error) {
			return nil, errors.New("")
		},
		func(repository string) ([]string, error) {
			return testTags, nil
		},
	)
	errFact := NewMockClientFactory(errClient, nil)
	reg := NewRegistry(errFact, log.NewNopLogger())
	_, err := reg.GetRepository(testRepository)
	if err == nil {
		t.Fatal("Expecting error")
	}
}

func TestRegistry_OrderByCreationDate(t *testing.T) {
	fmt.Printf("testTime: %s\n", testTime)
	time0 := testTime.Add(time.Second)
	time2 := testTime.Add(-time.Second)
	imA, _ := flux.ParseImage("my/Image:3", testTime)
	imB, _ := flux.ParseImage("my/Image:1", time0)
	imC, _ := flux.ParseImage("my/Image:4", time2)
	imD, _ := flux.ParseImage("my/Image:0", time.Time{}) // test nil
	imE, _ := flux.ParseImage("my/Image:2", testTime)    // test equal
	imgs := []flux.Image{imA, imB, imC, imD, imE}
	sort.Sort(byCreatedDesc(imgs))
	for i, im := range imgs {
		if strconv.Itoa(i) != im.ID.Tag {
			for j, jim := range imgs {
				t.Logf("%v: %v %s", j, jim.ID.String(), jim.CreatedAt)
			}
			t.Fatalf("Not sorted in expected order: %#v", imgs)
		}
	}
}
