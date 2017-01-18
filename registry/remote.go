package registry

import (
	"context"
	"encoding/json"
	manifest "github.com/docker/distribution/manifest/schema1"
	dockerregistry "github.com/heroku/docker-registry-client/registry"
	"time"
)

type Remote interface {
	Tags(img Image) ([]string, error)
	Manifest(img Image) (Image, error)
	Cancel()
}

type remote struct {
	client dockerRegistryInterface
	cancel context.CancelFunc
}

func newRemote(client *dockerregistry.Registry, cancel context.CancelFunc) Remote {
	return &remote{
		client: client,
		cancel: cancel,
	}
}

func (rc *remote) Tags(id Image) (_ []string, err error) {
	return rc.client.Tags(id.NamespaceImage())
}

func (rc *remote) Manifest(img Image) (Image, error) {
	meta, err := rc.client.Manifest(img.NamespaceImage(), img.Tag)
	if err != nil {
		return img, err
	}
	// the manifest includes some v1-backwards-compatibility data,
	// oddly called "History", which are layer metadata as JSON
	// strings; these appear most-recent (i.e., topmost layer) first,
	// so happily we can just decode the first entry to get a created
	// time.
	type v1image struct {
		Created time.Time `json:"created"`
	}
	var topmost v1image
	if err = json.Unmarshal([]byte(meta.History[0].V1Compatibility), &topmost); err == nil {
		if !topmost.Created.IsZero() {
			img.CreatedAt = &topmost.Created
		}
	}

	return img, err
}

func (rc *remote) Cancel() {
	rc.cancel()
}

// We need this because they didn't wrap it in an interface.
type dockerRegistryInterface interface {
	Tags(repository string) ([]string, error)
	Manifest(repository, reference string) (*manifest.SignedManifest, error)
}
