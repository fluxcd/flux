package cache

import (
	"testing"
)

func TestCache_Keys(t *testing.T) {
	u := "user"
	repo := "index.docker.io/library/alpine"
	key, err := NewManifestKey(u, repo, "tag")
	if err != nil {
		t.Fatal(err)
	}
	if key.String() != "registryhistoryv1|user|index.docker.io/library/alpine|tag" {
		t.Fatalf("Key doesn't match expected format: %q", key.String())
	}

	_, err = NewManifestKey(u, "not-full/path", "tag")
	if err == nil {
		t.Fatal("Expected error but got none")
	}
}
