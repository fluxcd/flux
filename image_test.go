package flux

import (
	"fmt"
	"testing"
)

func TestImageID_ParseImageID(t *testing.T) {
	for _, x := range []struct {
		test     string
		expected string
	}{
		{"alpine", "alpine:latest"},
		{"library/alpine", "alpine:latest"},
		{"alpine:mytag", "alpine:mytag"},
		{"quay.io/library/alpine", "quay.io/library/alpine:latest"},
		{"quay.io/library/alpine:latest", "quay.io/library/alpine:latest"},
		{"quay.io/library/alpine:mytag", "quay.io/library/alpine:mytag"},
	} {
		i, err := ParseImageID(x.test)
		if err != nil {
			t.Fatalf("Failed parsing %q, expected %q", x.test, x.expected)
		}
		if i.String() != x.expected {
			t.Fatalf("%q does not match expected %q", i.String(), x.expected)
		}
	}
}

func TestImageID_ParseImageIDErrorCases(t *testing.T) {
	for _, x := range []struct {
		test string
	}{
		{""},
		{":tag"},
		{"alpine::"},
		{"alpine:invalid:"},
		{"/too/many/slashes/"},
	} {
		_, err := ParseImageID(x.test)
		if err == nil {
			t.Fatalf("Expected parse failure for %q", x.test)
		}
	}
}

func TestImageID_TestComponents(t *testing.T) {
	host := "quay.io"
	namespace := "namespace"
	image := "myrepo"
	tag := "mytag"
	fqn := fmt.Sprintf("%v/%v/%v:%v", host, namespace, image, tag)
	i, err := ParseImageID(fqn)
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
