package cache

import (
	"strings"
	"time"

	"github.com/fluxcd/flux/pkg/image"
)

type Reader interface {
	// GetKey gets the value at a key, along with its refresh deadline
	GetKey(k Keyer) ([]byte, time.Time, error)
}

type Writer interface {
	// SetKey sets the value at a key, along with its refresh deadline
	SetKey(k Keyer, deadline time.Time, v []byte) error
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
		"registryhistoryv3", // Bump the version number if the cache format changes
		k.fullRepositoryPath,
		k.reference,
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
		"registryrepov4", // Bump the version number if the cache format changes
		k.fullRepositoryPath,
	}, "|")
}
