package registry

import (
	"fmt"
	"testing"
)

func TestImage_ParseImage(t *testing.T) {
	for _, x := range []struct {
		test     string
		expected string
	}{
		{"alpine", "index.docker.io/library/alpine:latest"},
		{"library/alpine", "index.docker.io/library/alpine:latest"},
		{"quay.io/library/alpine", "quay.io/library/alpine:latest"},
		{"quay.io/library/alpine:latest", "quay.io/library/alpine:latest"},
		{"quay.io/library/alpine:mytag", "quay.io/library/alpine:mytag"},
	} {
		i, err := ParseImage(x.test, nil)
		if err != nil {
			t.Fatalf("Failed parsing %q, expected %q", x.test, x.expected)
		}
		if i.String() != x.expected {
			t.Fatalf("%q does not match expected %q", i.String(), x.expected)
		}
	}
}

func TestImage_ParseImageErrorCases(t *testing.T) {
	for _, x := range []struct {
		test string
	}{
		{""},
		{"alpine::"},
		{"alpine:invalid:"},
		{"/too/many/slashes/"},
	} {
		_, err := ParseImage(x.test, nil)
		if err == nil {
			t.Fatalf("Expected parse failure for %q", x.test)
		}
	}
}

func TestImage_TestComponents(t *testing.T) {
	host := "quay.io"
	namespace := "namespace"
	image := "myrepo"
	tag := "mytag"
	fqn := fmt.Sprintf("%v/%v/%v:%v", host, namespace, image, tag)
	i, err := ParseImage(fqn, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range []struct {
		test     string
		expected string
	}{
		{i.Host, host},
		{i.Namespace, namespace},
		{i.Image, image},
		{i.Tag, tag},
		{i.String(), fqn},
	} {
		if x.test != x.expected {
			t.Fatalf("Expected %v, but got %v", x.expected, x.test)
		}
	}

}
