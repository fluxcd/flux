package registry

import (
	"github.com/docker/distribution/manifest/schema1"
	dockerregistry "github.com/heroku/docker-registry-client/registry"
	"github.com/pkg/errors"
	"testing"
	"time"
)

const testTagStr = "tag"
const testImageStr = "index.docker.io/test/Image:" + testTagStr
const constTime = "2017-01-13T16:22:58.009923189Z"

var (
	img, _         = ParseImage(testImageStr, nil)
	testRepository = RepositoryFromImage(img)
)

// Need to create a dummy manifest here
func TestRemoteClient_ParseManifest(t *testing.T) {
	man := schema1.SignedManifest{
		Manifest: schema1.Manifest{
			History: []schema1.History{
				{
					V1Compatibility: `{"created":"` + constTime + `"}`,
				},
			},
		},
	}
	c := remote{
		client: NewMockDockerClient(man, nil, nil),
	}
	testRepository = RepositoryFromImage(img)
	desc, err := c.Manifest(testRepository, img.Tag)
	if err != nil {
		t.Fatal(err.Error())
	}
	if string(desc.String()) != testImageStr {
		t.Fatalf("Expecting %q but got %q", testImageStr, string(desc.String()))
	}
	if desc.CreatedAt.Format(time.RFC3339Nano) != constTime {
		t.Fatalf("Expecting %q but got %q", constTime, desc.CreatedAt.Format(time.RFC3339Nano))
	}
}

// Just a simple pass through.
func TestRemoteClient_GetTags(t *testing.T) {
	c := remote{
		client: NewMockDockerClient(schema1.SignedManifest{}, []string{
			testTagStr,
		}, nil),
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

func TestRemoteClient_ErrorsForCoverage(t *testing.T) {
	c := remote{
		client: NewMockDockerClient(schema1.SignedManifest{}, []string{
			testTagStr,
		}, errors.New("dummy")),
	}
	_, err := c.Tags(testRepository)
	if err == nil {
		t.Fatal("Expected error")
	}
	_, err = c.Manifest(testRepository, img.Tag)
	if err == nil {
		t.Fatal("Expected error")
	}
}

func TestRemoteClient_TestNew(t *testing.T) {
	r := &dockerregistry.Registry{}
	var flag bool
	f := func() { flag = true }
	c := newRemote(r, f)
	if c.(*remote).client != r {
		t.Log("Client was not set")
	}
	c.(*remote).cancel()
	if !flag {
		t.Fatal("Expected it to call the cancel func")
	}
}
