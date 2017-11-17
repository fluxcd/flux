package registry

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/registry/client"
	"github.com/opencontainers/go-digest"

	"github.com/weaveworks/flux/image"
)

// Client is a remote registry client for a particular image
// repository (e.g., for quay.io/weaveworks/flux). It is an interface
// so we can wrap it in instrumentation, write fake implementations,
// and so on.
type Client interface {
	Tags(context.Context) ([]string, error)
	Manifest(ctx context.Context, ref string) (image.Info, error)
}

type Remote struct {
	transport http.RoundTripper
	repo      image.CanonicalName
}

// Adapt to docker distribution `reference.Named`.
type named struct {
	image.CanonicalName
}

// Name returns the name of the repository. These values are used to
// build API URLs, and (it turns out) are _not_ expected to include a
// domain (e.g., quay.io). Hence, the implementation here just returns
// the path.
func (n named) Name() string {
	return n.Image
}

// Return the tags for this repository.
func (a *Remote) Tags(ctx context.Context) ([]string, error) {
	repository, err := client.NewRepository(named{a.repo}, "https://"+a.repo.Domain, a.transport)
	if err != nil {
		return nil, err
	}
	return repository.Tags(ctx).All(ctx)
}

// Manifest fetches the metadata for an image reference; currently
// assumed to be in the same repo as that provided to `NewRemote(...)`
func (a *Remote) Manifest(ctx context.Context, ref string) (image.Info, error) {
	repository, err := client.NewRepository(named{a.repo}, "https://"+a.repo.Domain, a.transport)
	if err != nil {
		return image.Info{}, err
	}
	manifests, err := repository.Manifests(ctx)
	if err != nil {
		return image.Info{}, err
	}
	var manifestDigest digest.Digest
	digestOpt := client.ReturnContentDigest(&manifestDigest)
	manifest, fetchErr := manifests.Get(ctx, digest.Digest(ref), digestOpt, distribution.WithTagOption{ref})

interpret:
	if fetchErr != nil {
		return image.Info{}, err
	}

	info := image.Info{ID: a.repo.ToRef(ref), Digest: manifestDigest.String()}

	// TODO(michael): can we type switch? Not sure how dependable the
	// underlying types are.
	switch deserialised := manifest.(type) {
	case *schema1.SignedManifest:
		var man schema1.Manifest = deserialised.Manifest
		// for decoding the v1-compatibility entry in schema1 manifests
		var v1 struct {
			ID      string    `json:"id"`
			Created time.Time `json:"created"`
			OS      string    `json:"os"`
			Arch    string    `json:"architecture"`
		}

		if err = json.Unmarshal([]byte(man.History[0].V1Compatibility), &v1); err != nil {
			return image.Info{}, err
		}
		// This is not the ImageID that Docker uses, but assumed to
		// identify the image as it's the topmost layer.
		info.ImageID = v1.ID
		info.CreatedAt = v1.Created
	case *schema2.DeserializedManifest:
		var man schema2.Manifest = deserialised.Manifest
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
		// This _is_ what Docker uses as its Image ID.
		info.ImageID = man.Config.Digest.String()
		info.CreatedAt = config.Created
	case *manifestlist.DeserializedManifestList:
		var list manifestlist.ManifestList = deserialised.ManifestList
		// TODO(michael): is it valid to just pick the first one that matches?
		for _, m := range list.Manifests {
			if m.Platform.OS == "linux" && m.Platform.Architecture == "amd64" {
				manifest, fetchErr = manifests.Get(ctx, m.Digest, digestOpt)
				goto interpret
			}
		}
		return image.Info{}, errors.New("no suitable manifest (linux amd64) in manifestlist")
	default:
		t := reflect.TypeOf(manifest)
		return image.Info{}, errors.New("unknown manifest type: " + t.String())
	}
	return info, nil
}
