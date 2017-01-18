package registry

import (
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"sort"
	"strconv"
	"testing"
	"time"
)

var (
	testTags            = []string{testTagStr, "anotherTag"}
	mRemote             = NewMockRemote(img, testTags, nil)
	mRemoteFact         = NewMockRemoteFactory(mRemote, nil)
	testRegistryMetrics = NewMetrics().WithInstanceID("1")
	testTime, _         = time.Parse(constTime, time.RFC3339Nano)
)

func TestRegistry_GetImage(t *testing.T) {
	reg := NewRegistry(mRemoteFact, log.NewNopLogger(), testRegistryMetrics)
	newImg, err := reg.GetImage(img)
	if err != nil {
		t.Fatal(err)
	}
	if img.String() != newImg.String() {
		t.Fatal("Expected %v, but got %v", img.String(), newImg.String())
	}
}

func TestRegistry_GetImageFactoryErr(t *testing.T) {
	errFact := NewMockRemoteFactory(mRemote, errors.New(""))
	reg := NewRegistry(errFact, nil, testRegistryMetrics)
	_, err := reg.GetImage(img)
	if err == nil {
		t.Fatal("Expecting error")
	}
}

func TestRegistry_GetImageRemoteErr(t *testing.T) {
	r := NewMockRemote(img, testTags, errors.New(""))
	errFact := NewMockRemoteFactory(r, nil)
	reg := NewRegistry(errFact, log.NewNopLogger(), testRegistryMetrics)
	_, err := reg.GetImage(img)
	if err == nil {
		t.Fatal("Expecting error")
	}
}

func TestRegistry_GetRepository(t *testing.T) {
	reg := NewRegistry(mRemoteFact, log.NewNopLogger(), testRegistryMetrics)
	imgs, err := reg.GetRepository(img)
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
	errFact := NewMockRemoteFactory(mRemote, errors.New(""))
	reg := NewRegistry(errFact, nil, testRegistryMetrics)
	_, err := reg.GetRepository(img)
	if err == nil {
		t.Fatal("Expecting error")
	}
}

func TestRegistry_GetRepositoryRemoteErr(t *testing.T) {
	r := NewMockRemote(img, testTags, errors.New(""))
	errFact := NewMockRemoteFactory(r, nil)
	reg := NewRegistry(errFact, log.NewNopLogger(), testRegistryMetrics)
	_, err := reg.GetRepository(img)
	if err == nil {
		t.Fatal("Expecting error")
	}
}

func TestRegistry_GetRepositoryManifestError(t *testing.T) {
	r := NewMockRemote(img, []string{"valid", "error"}, nil)
	errFact := NewMockRemoteFactory(r, nil)
	reg := NewRegistry(errFact, log.NewNopLogger(), testRegistryMetrics)
	_, err := reg.GetRepository(img)
	if err == nil {
		t.Fatal("Expecting error")
	}
}

func TestRegistry_OrderByCreationDate(t *testing.T) {
	time0 := testTime.Add(time.Second)
	time2 := testTime.Add(-time.Second)
	imA, _ := ParseImage("my/Image:3", &testTime)
	imB, _ := ParseImage("my/Image:1", &time0)
	imC, _ := ParseImage("my/Image:4", &time2)
	imD, _ := ParseImage("my/Image:0", nil)       // test nil
	imE, _ := ParseImage("my/Image:2", &testTime) // test equal
	imgs := []Image{imA, imB, imC, imD, imE}
	sort.Sort(byCreatedDesc(imgs))
	for i, im := range imgs {
		if strconv.Itoa(i) != im.Tag {
			for j, jim := range imgs {
				t.Logf("%v: %v", j, jim.String())
			}
			t.Fatalf("Not sorted in expected order: %#v", imgs)
		}
	}
}
