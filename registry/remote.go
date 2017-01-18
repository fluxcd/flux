package registry

import (
	"context"
	"encoding/json"
	"fmt"
	manifest "github.com/docker/distribution/manifest/schema1"
	dockerregistry "github.com/heroku/docker-registry-client/registry"
	"time"
)

type Remote interface {
	Tags(repository Repository) ([]string, error)
	Manifest(repository Repository, tag string) (Image, error)
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

func (rc *remote) Tags(repository Repository) (_ []string, err error) {
	return rc.client.Tags(repository.NamespaceImage())
}

func (rc *remote) Manifest(repository Repository, tag string) (img Image, err error) {
	img, err = ParseImage(fmt.Sprintf("%s:%s", repository.String(), tag), nil)
	if err != nil {
		return
	}
	meta, err := rc.client.Manifest(repository.NamespaceImage(), tag)
	if err != nil {
		return
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

	return
}

func (rc *remote) Cancel() {
	rc.cancel()
}

// We need this because they didn't wrap it in an interface.
type dockerRegistryInterface interface {
	Tags(repository string) ([]string, error)
	Manifest(repository, reference string) (*manifest.SignedManifest, error)
}
