package flux

import (
	"encoding/json"
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

func TestImageID_Serialization(t *testing.T) {
	for _, x := range []struct {
		test     ImageID
		expected string
	}{
		{ImageID{Host: dockerHubHost, Namespace: dockerHubLibrary, Image: "alpine", Tag: "a123"}, `"alpine:a123"`},
		{ImageID{Host: "quay.io", Namespace: "weaveworks", Image: "foobar", Tag: "baz"}, `"quay.io/weaveworks/foobar:baz"`},
	} {
		serialized, err := json.Marshal(x.test)
		if err != nil {
			t.Fatalf("Error encoding %v: %v", x.test, err)
		}
		if string(serialized) != x.expected {
			t.Fatalf("Encoded %v as %s, but expected %s", x.test, string(serialized), x.expected)
		}

		var decoded ImageID
		if err := json.Unmarshal([]byte(x.expected), &decoded); err != nil {
			t.Fatalf("Error decoding %v: %v", x.expected, err)
		}
		if decoded != x.test {
			t.Fatalf("Decoded %s as %v, but expected %v", x.expected, decoded, x.test)
		}
	}
}
