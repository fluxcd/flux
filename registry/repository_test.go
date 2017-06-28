package registry

import "testing"

func TestRepository_ParseImage(t *testing.T) {
	for _, x := range []struct {
		test     string
		expected string
	}{
		{"alpine", "index.docker.io/library/alpine"},
		{"library/alpine", "index.docker.io/library/alpine"},
		{"alpine:mytag", "index.docker.io/library/alpine"},
		{"quay.io/library/alpine", "quay.io/library/alpine"},
		{"quay.io/library/alpine:latest", "quay.io/library/alpine"},
		{"quay.io/library/alpine:mytag", "quay.io/library/alpine"},
	} {
		i, err := ParseRepository(x.test)
		if err != nil {
			t.Fatalf("Failed parsing %q, expected %q", x.test, x.expected)
		}
		if i.String() != x.expected {
			t.Fatalf("%q does not match expected %q", i.String(), x.expected)
		}
	}
}
