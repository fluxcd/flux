package registry

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/registry/client"
	"github.com/opencontainers/go-digest"

	"github.com/weaveworks/flux/image"
)

type Remote struct {
	transport http.RoundTripper
	repo      image.CanonicalName
}

// Adapt to docker distribution reference.Named
type named struct {
	image.CanonicalName
}

func (n named) Name() string {
	return n.Image
}

func (n named) String() string {
	return n.String()
}

// Return the tags for this repository.
func (a *Remote) Tags() ([]string, error) {
	ctx := context.TODO()
	repository, err := client.NewRepository(named{a.repo}, "https://"+a.repo.Domain, a.transport)
	if err != nil {
		return nil, err
	}
	return repository.Tags(ctx).All(ctx)
}

// Manifest fetches the metadata for an image reference; currently
// assumed to be in the same repo as that provided to `NewRemote(...)`
func (a *Remote) Manifest(ref string) (image.Info, error) {
	ctx := context.TODO()
	repository, err := client.NewRepository(named{a.repo}, "https://"+a.repo.Domain, a.transport)
	if err != nil {
		return image.Info{}, err
	}
	manifests, err := repository.Manifests(ctx)
	if err != nil {
		return image.Info{}, err
	}
	manifest, fetchErr := manifests.Get(ctx, digest.Digest(ref), distribution.WithTagOption{ref})

interpret:
	if fetchErr != nil {
		return image.Info{}, err
	}

	mt, bytes, err := manifest.Payload()
	if err != nil {
		return image.Info{}, err
	}

	info := image.Info{ID: a.repo.ToRef(ref)}

	// for decoding the v1-compatibility entry in schema1 manifests
	var v1 struct {
		ID      string    `json:"id"`
		Created time.Time `json:"created"`
		OS      string    `json:"os"`
		Arch    string    `json:"architecture"`
	}

	// TODO(michael): can we type switch? Not sure how dependable the
	// underlying types are.
	switch mt {
	case schema1.MediaTypeManifest:
		// TODO: can this be fallthrough? Find something to check on...
		var man schema1.Manifest
		if err = json.Unmarshal(bytes, &man); err != nil {
			return image.Info{}, err
		}
		if err = json.Unmarshal([]byte(man.History[0].V1Compatibility), &v1); err != nil {
			return image.Info{}, err
		}
		info.CreatedAt = v1.Created
	case schema1.MediaTypeSignedManifest:
		var man schema1.SignedManifest
		if err = json.Unmarshal(bytes, &man); err != nil {
			return image.Info{}, err
		}
		if err = json.Unmarshal([]byte(man.History[0].V1Compatibility), &v1); err != nil {
			return image.Info{}, err
		}
		info.CreatedAt = v1.Created
	case schema2.MediaTypeManifest:
		var man schema2.Manifest
		if err = json.Unmarshal(bytes, &man); err != nil {
			return image.Info{}, err
		}

		configBytes, err := repository.Blobs(ctx).Get(ctx, man.Config.Digest)
		if err != nil {
			return image.Info{}, err
		}

		var config struct {
			Arch    string    `json:"architecture"`
			Created time.Time `json:"created"`
			OS      string    `json:"os"`
		}
		if err = json.Unmarshal(configBytes, &config); err != nil {
			return image.Info{}, err
		}
		info.CreatedAt = config.Created
	case manifestlist.MediaTypeManifestList:
		var list manifestlist.ManifestList
		if err = json.Unmarshal(bytes, &list); err != nil {
			return image.Info{}, err
		}
		// TODO(michael): can we just pick the first one that matches?
		for _, m := range list.Manifests {
			if m.Platform.OS == "linux" && m.Platform.Architecture == "amd64" {
				manifest, fetchErr = manifests.Get(ctx, m.Digest)
				goto interpret
			}
		}
		return image.Info{}, errors.New("no suitable manifest (linux amd64) in manifestlist")
	}
	return info, nil
}
