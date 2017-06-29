package registry

import (
	"context"
	"encoding/json"
	"time"

	"github.com/docker/distribution/manifest/schema1"

	"fmt"
	"github.com/weaveworks/flux"
)

type Remote interface {
	Tags(repository Repository) ([]string, error)
	Manifest(repository Repository, tag string) (flux.Image, error)
	Cancel()
}

type remote struct {
	client dockerRegistryInterface
	cancel context.CancelFunc
}

func newRemote(client dockerRegistryInterface, cancel context.CancelFunc) Remote {
	return &remote{
		client: client,
		cancel: cancel,
	}
}

func (rc *remote) Tags(repository Repository) (_ []string, err error) {
	return rc.client.Tags(repository.String())
}

func (rc *remote) Manifest(repository Repository, tag string) (img flux.Image, err error) {
	img, err = flux.ParseImage(fmt.Sprintf("%s:%s", repository.String(), tag), time.Time{})
	if err != nil {
		return
	}
	history, err := rc.client.Manifest(repository.String(), tag)
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
	if len(history) > 0 {
		if err = json.Unmarshal([]byte(history[0].V1Compatibility), &topmost); err == nil {
			if !topmost.Created.IsZero() {
				img.CreatedAt = topmost.Created
			}
		}
	}

	return
}

func (rc *remote) Cancel() {
	rc.cancel()
}

// This is an interface that represents the heroku docker registry library
type dockerRegistryInterface interface {
	Tags(repository string) ([]string, error)
	Manifest(repository, reference string) ([]schema1.History, error)
}
