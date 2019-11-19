package image

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

func TestParseRef(t *testing.T) {
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
		i, err := ParseRef(x.test)
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

func TestParseRefErrorCases(t *testing.T) {
	for _, x := range []struct {
		test string
	}{
		{""},
		{":tag"},
		{"/leading/slash"},
		{"trailing/slash/"},
	} {
		_, err := ParseRef(x.test)
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
	i, err := ParseRef(fqn)
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

func TestRefSerialization(t *testing.T) {
	for _, x := range []struct {
		test     Ref
		expected string
	}{
		{Ref{Name: Name{Image: "alpine"}, Tag: "a123"}, `"alpine:a123"`},
		{Ref{Name: Name{Domain: "quay.io", Image: "weaveworks/foobar"}, Tag: "baz"}, `"quay.io/weaveworks/foobar:baz"`},
	} {
		serialized, err := json.Marshal(x.test)
		if err != nil {
			t.Errorf("Error encoding %v: %v", x.test, err)
		}
		if string(serialized) != x.expected {
			t.Errorf("Encoded %v as %s, but expected %s", x.test, string(serialized), x.expected)
		}

		var decoded Ref
		if err := json.Unmarshal([]byte(x.expected), &decoded); err != nil {
			t.Errorf("Error decoding %v: %v", x.expected, err)
		}
		if decoded != x.test {
			t.Errorf("Decoded %s as %v, but expected %v", x.expected, decoded, x.test)
		}
	}
}

func TestImageLabelsSerialisation(t *testing.T) {
	t0 := time.Now().UTC() // UTC so it has nil location, otherwise it won't compare
	t1 := time.Now().Add(5 * time.Minute).UTC()
	labels := Labels{Created: t0, BuildDate: t1}
	bytes, err := json.Marshal(labels)
	if err != nil {
		t.Fatal(err)
	}
	var labels1 Labels
	if err = json.Unmarshal(bytes, &labels1); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, labels, labels1)
}

func TestNonRFC3339ImageLabelsUnmarshal(t *testing.T) {
	str := `{
	"org.label-schema.build-date": "20190523",
	"org.opencontainers.image.created": "20190523"
}`

	var labels Labels
	err := json.Unmarshal([]byte(str), &labels)
	lpe, ok := err.(*LabelTimestampFormatError)
	if !ok {
		t.Fatalf("Got %v, but expected LabelTimestampFormatError", err)
	}
	if lc := len(lpe.Labels); lc != 2 {
		t.Errorf("Got error for %v labels, expected 2", lc)
	}
}

func TestImageLabelsZeroTime(t *testing.T) {
	labels := Labels{}
	bytes, err := json.Marshal(labels)
	if err != nil {
		t.Fatal(err)
	}
	var labels1 map[string]interface{}
	if err = json.Unmarshal(bytes, &labels1); err != nil {
		t.Fatal(err)
	}
	if lc := len(labels1); lc >= 1 {
		t.Errorf("serialised Labels contains %v fields; expected it to contain none\n%v", lc, labels1)
	}
}

func mustMakeInfo(ref string, created time.Time) Info {
	r, err := ParseRef(ref)
	if err != nil {
		panic(err)
	}
	return Info{ID: r, CreatedAt: created}
}

func (im Info) setLabels(labels Labels) Info {
	im.Labels = labels
	return im
}

func TestImageInfoSerialisation(t *testing.T) {
	t0 := time.Now().UTC() // UTC so it has nil location, otherwise it won't compare
	t1 := time.Now().Add(5 * time.Minute).UTC()
	info := mustMakeInfo("my/image:tag", t0)
	info.Digest = "sha256:digest"
	info.ImageID = "sha256:layerID"
	info.LastFetched = t1
	bytes, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	var info1 Info
	if err = json.Unmarshal(bytes, &info1); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, info, info1)
}

func TestImageInfoCreatedAtZero(t *testing.T) {
	info := mustMakeInfo("my/image:tag", time.Now())
	info = Info{ID: info.ID}
	bytes, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	var info1 map[string]interface{}
	if err = json.Unmarshal(bytes, &info1); err != nil {
		t.Fatal(err)
	}
	if _, ok := info1["CreatedAt"]; ok {
		t.Errorf("serialised Info included zero time field; expected it to be omitted\n%s", string(bytes))
	}
}

func TestImage_OrderByCreationDate(t *testing.T) {
	time0 := testTime.Add(time.Second)
	time2 := testTime.Add(-time.Second)
	ignoredTime := testTime.Add(+time.Minute)
	ignoredTime2 := testTime.Add(+time.Hour)
	imA := mustMakeInfo("my/Image:2", testTime)
	imB := mustMakeInfo("my/Image:0", time0).setLabels(Labels{Created: ignoredTime2})  // label time should be ignored
	imC := mustMakeInfo("my/Image:3", time2).setLabels(Labels{BuildDate: ignoredTime}) // label time should be ignored
	imD := mustMakeInfo("my/Image:4", time.Time{})                                     // test nil
	imE := mustMakeInfo("my/Image:1", testTime).setLabels(Labels{Created: testTime})   // test equal
	imF := mustMakeInfo("my/Image:5", time.Time{})                                     // test nil equal
	imgs := []Info{imA, imB, imC, imD, imE, imF}
	Sort(imgs, NewerByCreated)
	checkSorted(t, imgs)
	// now check stability
	Sort(imgs, NewerByCreated)
	checkSorted(t, imgs)
	reverse(imgs)
	Sort(imgs, NewerByCreated)
	checkSorted(t, imgs)
}

func checkSorted(t *testing.T, imgs []Info) {
	for i, im := range imgs {
		if strconv.Itoa(i) != im.ID.Tag {
			for j, jim := range imgs {
				t.Logf("%v: %v %s", j, jim.ID.String(), jim.CreatedAt)
			}
			t.Fatalf("Not sorted in expected order: %#v", imgs)
		}
	}
}

func TestImage_OrderBySemverTagDesc(t *testing.T) {
	ti := time.Time{}
	aa := mustMakeInfo("my/image:3", ti)
	bb := mustMakeInfo("my/image:v1", ti)
	cc := mustMakeInfo("my/image:1.10", ti)
	dd := mustMakeInfo("my/image:1.2.30", ti)
	ee := mustMakeInfo("my/image:1.10.0", ti) // same as 1.10 but should be considered newer
	ff := mustMakeInfo("my/image:bbb-not-semver", ti)
	gg := mustMakeInfo("my/image:aaa-not-semver", ti)

	imgs := []Info{aa, bb, cc, dd, ee, ff, gg}
	Sort(imgs, NewerBySemver)

	expected := []Info{aa, ee, cc, dd, bb, gg, ff}
	assert.Equal(t, tags(expected), tags(imgs))

	// stable?
	reverse(imgs)
	Sort(imgs, NewerBySemver)
	assert.Equal(t, tags(expected), tags(imgs))
}

func tags(imgs []Info) []string {
	var vs []string
	for _, i := range imgs {
		vs = append(vs, i.ID.Tag)
	}
	return vs
}

func reverse(imgs []Info) {
	for i := len(imgs)/2 - 1; i >= 0; i-- {
		opp := len(imgs) - 1 - i
		imgs[i], imgs[opp] = imgs[opp], imgs[i]
	}
}
