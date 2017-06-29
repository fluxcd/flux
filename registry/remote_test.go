package registry

import (
	"testing"
	"time"

	"github.com/docker/distribution/manifest/schema1"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
)

const testTagStr = "tag"
const testImageStr = "index.docker.io/test/Image:" + testTagStr
const constTime = "2017-01-13T16:22:58.009923189Z"

var (
	img, _         = flux.ParseImage(testImageStr, time.Time{})
	testRepository = RepositoryFromImage(img)

	man = schema1.SignedManifest{
		Manifest: schema1.Manifest{
			History: []schema1.History{
				{
					V1Compatibility: `{"created":"` + constTime + `"}`,
				},
			},
		},
	}
)

// Need to create a dummy manifest here
func TestRemoteClient_ParseManifest(t *testing.T) {
	manifestFunc := func(repo, ref string) ([]schema1.History, error) {
		return man.Manifest.History, nil
	}
	c := newRemote(
		NewMockDockerClient(manifestFunc, nil),
		nil,
	)
	testRepository = RepositoryFromImage(img)
	desc, err := c.Manifest(testRepository, img.ID.Tag)
	if err != nil {
		t.Fatal(err.Error())
	}
	if string(desc.ID.FullID()) != testImageStr {
		t.Fatalf("Expecting %q but got %q", testImageStr, string(desc.ID.FullID()))
	}
	if desc.CreatedAt.Format(time.RFC3339Nano) != constTime {
		t.Fatalf("Expecting %q but got %q", constTime, desc.CreatedAt.Format(time.RFC3339Nano))
	}
}

// Just a simple pass through.
func TestRemoteClient_GetTags(t *testing.T) {
	c := remote{
		client: NewMockDockerClient(nil, func(repository string) ([]string, error) {
			return []string{
				testTagStr,
			}, nil
		}),
	}
	tags, err := c.Tags(testRepository)
	if err != nil {
		t.Fatal(err.Error())
	}
	if tags[0] != testTagStr {
		t.Fatalf("Expecting %q but got %q", testTagStr, tags[0])
	}
}

func TestRemoteClient_IsCancelCalled(t *testing.T) {
	var didCancel bool
	r := remote{
		cancel: func() { didCancel = true },
	}
	r.Cancel()
	if !didCancel {
		t.Fatal("Expected it to call the cancel func")
	}
}

func TestRemoteClient_RemoteErrors(t *testing.T) {
	manifestFunc := func(repo, ref string) ([]schema1.History, error) {
		return man.Manifest.History, errors.New("dummy")
	}
	tagsFunc := func(repository string) ([]string, error) {
		return []string{
			testTagStr,
		}, errors.New("dummy")
	}
	c := remote{
		client: NewMockDockerClient(manifestFunc, tagsFunc),
	}
	_, err := c.Tags(testRepository)
	if err == nil {
		t.Fatal("Expected error")
	}
	_, err = c.Manifest(testRepository, img.ID.Tag)
	if err == nil {
		t.Fatal("Expected error")
	}
}

func TestRemoteClient_TestNew(t *testing.T) {
	r := &herokuWrapper{}
	var flag bool
	f := func() { flag = true }
	c := newRemote(r, f)
	if c.(*remote).client != r { // Test that client was set
		t.Fatal("Client was not set")
	}
	c.(*remote).cancel()
	if !flag { // Test that our cancel function, when called, works
		t.Fatal("Expected it to call the cancel func")
	}
}
