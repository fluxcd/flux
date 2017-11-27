package cache

import (
	"strings"
	"time"

	"github.com/weaveworks/flux/image"
)

type Reader interface {
	GetKey(k Keyer) ([]byte, time.Time, error)
}

type Writer interface {
	SetKey(k Keyer, v []byte) error
}

type Client interface {
	Reader
	Writer
}

// An interface to provide the key under which to store the data
// Use the full path to image for the memcache key because there
// might be duplicates from other registries
type Keyer interface {
	Key() string
}

type manifestKey struct {
	fullRepositoryPath, reference string
}

func NewManifestKey(image image.CanonicalRef) Keyer {
	return &manifestKey{image.CanonicalName().String(), image.Tag}
}

func (k *manifestKey) Key() string {
	return strings.Join([]string{
		"registryhistoryv3", // Just to version in case we need to change format later.
		k.fullRepositoryPath,
		k.reference,
	}, "|")
}

type tagKey struct {
	fullRepositoryPath string
}

func NewTagKey(id image.CanonicalName) Keyer {
	return &tagKey{id.String()}
}

func (k *tagKey) Key() string {
	return strings.Join([]string{
		"registrytagsv3", // Just to version in case we need to change format later.
		k.fullRepositoryPath,
	}, "|")
}

type repoKey struct {
	fullRepositoryPath string
}

func NewRepositoryKey(repo image.CanonicalName) Keyer {
	return &repoKey{repo.String()}
}

func (k *repoKey) Key() string {
	return strings.Join([]string{
		"registryrepov3",
		k.fullRepositoryPath,
	}, "|")
}
