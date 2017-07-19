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
