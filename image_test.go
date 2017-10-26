package flux

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"testing"
	"time"
)

const constTime = "2017-01-13T16:22:58.009923189Z"

var (
	testTime, _ = time.Parse(time.RFC3339Nano, constTime)
)

func TestDomainRegexp(t *testing.T) {
	for _, d := range []string{
		"localhost", "localhost:5000",
		"example.com", "example.com:80",
		"gcr.io",
		"index.docker.com",
	} {
		if !domainRegexp.MatchString(d) {
			t.Errorf("domain regexp did not match %q", d)
		}
	}
}

func TestParseImageRef(t *testing.T) {
	for _, x := range []struct {
		test     string
		registry string
		repo     string
		canon    string
	}{
		// Library images can have the domain omitted; a
		// single-element path is understood to be prefixed with "library".
		{"alpine", dockerHubHost, "library/alpine", "index.docker.io/library/alpine"},
		{"library/alpine", dockerHubHost, "library/alpine", "index.docker.io/library/alpine"},
		{"alpine:mytag", dockerHubHost, "library/alpine", "index.docker.io/library/alpine:mytag"},
		// The old registry path should be replaced with the new one
		{"docker.io/library/alpine", dockerHubHost, "library/alpine", "index.docker.io/library/alpine"},
		// It's possible to have a domain with a single-element path
		{"localhost/hello:v1.1", "localhost", "hello", "localhost/hello:v1.1"},
		{"localhost:5000/hello:v1.1", "localhost:5000", "hello", "localhost:5000/hello:v1.1"},
		{"example.com/hello:v1.1", "example.com", "hello", "example.com/hello:v1.1"},
		// The path can have an arbitrary number of elements
		{"quay.io/library/alpine", "quay.io", "library/alpine", "quay.io/library/alpine"},
		{"quay.io/library/alpine:latest", "quay.io", "library/alpine", "quay.io/library/alpine:latest"},
		{"quay.io/library/alpine:mytag", "quay.io", "library/alpine", "quay.io/library/alpine:mytag"},
		{"localhost:5000/path/to/repo/alpine:mytag", "localhost:5000", "path/to/repo/alpine", "localhost:5000/path/to/repo/alpine:mytag"},
	} {
		i, err := ParseImageRef(x.test)
		if err != nil {
			t.Errorf("Failed parsing %q: %s", x.test, err)
		}
		if i.String() != x.test {
			t.Errorf("%q does not stringify as itself; got %q", x.test, i.String())
		}
		if i.Registry() != x.registry {
			t.Errorf("%q registry: expected %q, got %q", x.test, x.registry, i.Registry())
		}
		if i.Repository() != x.repo {
			t.Errorf("%q repo: expected %q, got %q", x.test, x.repo, i.Repository())
		}
		if i.CanonicalRef().String() != x.canon {
			t.Errorf("%q full ID: expected %q, got %q", x.test, x.canon, i.CanonicalRef().String())
		}
	}
}

func TestParseImageRefErrorCases(t *testing.T) {
	for _, x := range []struct {
		test string
	}{
		{""},
		{":tag"},
		{"/leading/slash"},
		{"trailing/slash/"},
	} {
		_, err := ParseImageRef(x.test)
		if err == nil {
			t.Fatalf("Expected parse failure for %q", x.test)
		}
	}
}

func TestComponents(t *testing.T) {
	host := "quay.io"
	image := "my/repo"
	tag := "mytag"
	fqn := fmt.Sprintf("%v/%v:%v", host, image, tag)
	i, err := ParseImageRef(fqn)
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range []struct {
		test     string
		expected string
	}{
		{i.Domain, host},
		{i.Image, image},
		{i.Tag, tag},
		{i.String(), fqn},
	} {
		if x.test != x.expected {
			t.Fatalf("Expected %v, but got %v", x.expected, x.test)
		}
	}
}

func TestImageRefSerialization(t *testing.T) {
	for _, x := range []struct {
		test     ImageRef
		expected string
	}{
		{ImageRef{ImageName: ImageName{Image: "alpine"}, Tag: "a123"}, `"alpine:a123"`},
		{ImageRef{ImageName: ImageName{Domain: "quay.io", Image: "weaveworks/foobar"}, Tag: "baz"}, `"quay.io/weaveworks/foobar:baz"`},
	} {
		serialized, err := json.Marshal(x.test)
		if err != nil {
			t.Errorf("Error encoding %v: %v", x.test, err)
		}
		if string(serialized) != x.expected {
			t.Errorf("Encoded %v as %s, but expected %s", x.test, string(serialized), x.expected)
		}

		var decoded ImageRef
		if err := json.Unmarshal([]byte(x.expected), &decoded); err != nil {
			t.Errorf("Error decoding %v: %v", x.expected, err)
		}
		if decoded != x.test {
			t.Errorf("Decoded %s as %v, but expected %v", x.expected, decoded, x.test)
		}
	}
}

func TestImage_OrderByCreationDate(t *testing.T) {
	fmt.Printf("testTime: %s\n", testTime)
	time0 := testTime.Add(time.Second)
	time2 := testTime.Add(-time.Second)
	imA, _ := ParseImage("my/Image:3", testTime)
	imB, _ := ParseImage("my/Image:1", time0)
	imC, _ := ParseImage("my/Image:4", time2)
	imD, _ := ParseImage("my/Image:0", time.Time{}) // test nil
	imE, _ := ParseImage("my/Image:2", testTime)    // test equal
	imgs := []Image{imA, imB, imC, imD, imE}
	sort.Sort(ByCreatedDesc(imgs))
	for i, im := range imgs {
		if strconv.Itoa(i) != im.ID.Tag {
			for j, jim := range imgs {
				t.Logf("%v: %v %s", j, jim.ID.String(), jim.CreatedAt)
			}
			t.Fatalf("Not sorted in expected order: %#v", imgs)
		}
	}
}
