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

	"github.com/fluxcd/flux/pkg/image"
)

type Excluded struct {
	ExcludedReason string `json:",omitempty"`
}

// ImageEntry represents a result from looking up an image ref in an
// image registry. It's an either-or: either you get an image.Info, or
// you get a reason that the image should be treated as unusable
// (e.g., it's for the wrong architecture).
type ImageEntry struct {
	image.Info `json:",omitempty"`
	Excluded
}

// MarshalJSON does custom JSON marshalling for ImageEntry values. We
// need this because the struct embeds the image.Info type, which has
// its own custom marshaling, which would get used otherwise.
func (entry ImageEntry) MarshalJSON() ([]byte, error) {
	// We can only do it this way because it's explicitly an either-or
	// -- I don't know of a way to inline all the fields when one of
	// the things you're inlining defines its own MarshalJSON.
	if entry.ExcludedReason != "" {
		return json.Marshal(entry.Excluded)
	}
	return json.Marshal(entry.Info)
}

// UnmarshalJSON does custom JSON unmarshalling for ImageEntry values.
func (entry *ImageEntry) UnmarshalJSON(bytes []byte) error {
	if err := json.Unmarshal(bytes, &entry.Info); err != nil {
		return err
	}
	if err := json.Unmarshal(bytes, &entry.Excluded); err != nil {
		return err
	}
	return nil
}

// Client is a remote registry client for a particular image
// repository (e.g., for docker.io/fluxcd/flux). It is an interface
// so we can wrap it in instrumentation, write fake implementations,
// and so on.
type Client interface {
	Tags(context.Context) ([]string, error)
	Manifest(ctx context.Context, ref string) (ImageEntry, error)
}

// ClientFactory supplies Client implementations for a given repo,
// with credentials. This is an interface so we can provide fake
// implementations.
type ClientFactory interface {
	ClientFor(image.CanonicalName, Credentials) (Client, error)
	Succeed(image.CanonicalName)
}

type Remote struct {
	transport http.RoundTripper
	repo      image.CanonicalName
	base      string
}

// Adapt to docker distribution `reference.Named`.
type named struct {
	image.CanonicalName
}

// Name returns the name of the repository. These values are used by
// the docker distribution client package to build API URLs, and (it
// turns out) are _not_ expected to include a domain (e.g.,
// quay.io). Hence, the implementation here just returns the path.
func (n named) Name() string {
	return n.Image
}

// Return the tags for this repository.
func (a *Remote) Tags(ctx context.Context) ([]string, error) {
	repository, err := client.NewRepository(named{a.repo}, a.base, a.transport)
	if err != nil {
		return nil, err
	}
	return repository.Tags(ctx).All(ctx)
}

// Manifest fetches the metadata for an image reference; currently
// assumed to be in the same repo as that provided to `NewRemote(...)`
func (a *Remote) Manifest(ctx context.Context, ref string) (ImageEntry, error) {
	repository, err := client.NewRepository(named{a.repo}, a.base, a.transport)
	if err != nil {
		return ImageEntry{}, err
	}
	manifests, err := repository.Manifests(ctx)
	if err != nil {
		return ImageEntry{}, err
	}
	var manifestDigest digest.Digest
	digestOpt := client.ReturnContentDigest(&manifestDigest)
	manifest, fetchErr := manifests.Get(ctx, digest.Digest(ref), digestOpt, distribution.WithTagOption{ref})

interpret:
	if fetchErr != nil {
		return ImageEntry{}, fetchErr
	}

	var labelErr error
	info := image.Info{ID: a.repo.ToRef(ref), Digest: manifestDigest.String()}

	// TODO(michael): can we type switch? Not sure how dependable the
	// underlying types are.
	switch deserialised := manifest.(type) {
	case *schema1.SignedManifest:
		var man schema1.Manifest = deserialised.Manifest
		// For decoding the v1-compatibility entry in schema1 manifests
		// Ref: https://docs.docker.com/registry/spec/manifest-v2-1/
		// Ref (spec): https://github.com/moby/moby/blob/master/image/spec/v1.md#image-json-field-descriptions
		var v1 struct {
			ID      string    `json:"id"`
			Created time.Time `json:"created"`
			OS      string    `json:"os"`
			Arch    string    `json:"architecture"`
		}
		if err = json.Unmarshal([]byte(man.History[0].V1Compatibility), &v1); err != nil {
			return ImageEntry{}, err
		}

		var config struct {
			Config  struct {
				Labels image.Labels `json:"labels"`
			} `json:"config"`
		}
		// We need to unmarshal the labels separately as the validation error
		// that may be returned stops the unmarshalling which would result
		// in no data at all for the image.
		if err = json.Unmarshal([]byte(man.History[0].V1Compatibility), &config); err != nil {
			if _, ok := err.(*image.LabelTimestampFormatError); !ok {
				return ImageEntry{}, err
			}
			labelErr = err
		}

		// This is not the ImageID that Docker uses, but assumed to
		// identify the image as it's the topmost layer.
		info.ImageID = v1.ID
		info.CreatedAt = v1.Created
		info.Labels = config.Config.Labels
	case *schema2.DeserializedManifest:
		var man schema2.Manifest = deserialised.Manifest
		configBytes, err := repository.Blobs(ctx).Get(ctx, man.Config.Digest)
		if err != nil {
			return ImageEntry{}, err
		}

		// Ref: https://github.com/docker/distribution/blob/master/docs/spec/manifest-v2-2.md
		var config struct {
			Arch            string    `json:"architecture"`
			Created         time.Time `json:"created"`
			OS              string    `json:"os"`
		}
		if err = json.Unmarshal(configBytes, &config); err != nil {
			return ImageEntry{}, nil
		}

		// Ref: https://github.com/moby/moby/blob/39e6def2194045cb206160b66bf309f486bd7e64/image/image.go#L47
		var container struct {
			ContainerConfig struct {
				Labels image.Labels `json:"labels"`
			} `json:"container_config"`
		}
		// We need to unmarshal the labels separately as the validation error
		// that may be returned stops the unmarshalling which would result
		// in no data at all for the image.
		if err = json.Unmarshal(configBytes, &container); err != nil {
			if _, ok := err.(*image.LabelTimestampFormatError); !ok {
				return ImageEntry{}, err
			}
			labelErr = err
		}

		// This _is_ what Docker uses as its Image ID.
		info.ImageID = man.Config.Digest.String()
		info.CreatedAt = config.Created
		info.Labels = container.ContainerConfig.Labels
	case *manifestlist.DeserializedManifestList:
		var list manifestlist.ManifestList = deserialised.ManifestList
		// TODO(michael): is it valid to just pick the first one that matches?
		for _, m := range list.Manifests {
			if m.Platform.OS == "linux" && m.Platform.Architecture == "amd64" {
				manifest, fetchErr = manifests.Get(ctx, m.Digest, digestOpt)
				goto interpret
			}
		}
		entry := ImageEntry{}
		entry.ExcludedReason = "no suitable manifest (linux amd64) in manifestlist"
		return entry, nil
	default:
		t := reflect.TypeOf(manifest)
		return ImageEntry{}, errors.New("unknown manifest type: " + t.String())
	}
	return ImageEntry{Info: info}, labelErr
}
