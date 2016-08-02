package registry

import "testing"

func TestParseImage(t *testing.T) {
	for _, input := range []struct {
		in  string
		out Image
	}{
		{
			"foo/bar",
			Image{Registry: "", Name: "foo/bar", Tag: ""},
		},
		{
			"foo/bar:baz",
			Image{Registry: "", Name: "foo/bar", Tag: "baz"},
		},
		{
			"reg:123/foo/bar:baz",
			Image{Registry: "reg:123", Name: "foo/bar", Tag: "baz"},
		},
		{
			"docker-registry.domain.name:5000/repo/image1:ver",
			Image{Registry: "docker-registry.domain.name:5000", Name: "repo/image1", Tag: "ver"},
		},
		{
			"shortreg/repo/image1",
			Image{Registry: "shortreg", Name: "repo/image1", Tag: ""},
		},
		{
			"foo",
			Image{Registry: "", Name: "foo", Tag: ""},
		},
	} {
		out := ParseImage(input.in)
		if out != input.out {
			t.Fatalf("%s: %v != %v", input.in, out, input.out)
		}
	}
}

func TestImageString(t *testing.T) {
	for _, input := range []struct {
		in  Image
		out string
	}{
		{
			Image{Registry: "", Name: "foo/bar", Tag: ""},
			"foo/bar",
		},
		{
			Image{Registry: "", Name: "foo/bar", Tag: "baz"},
			"foo/bar:baz",
		},
		{
			Image{Registry: "reg:123", Name: "foo/bar", Tag: "baz"},
			"reg:123/foo/bar:baz",
		},
		{
			Image{Registry: "docker-registry.domain.name:5000", Name: "repo/image1", Tag: "ver"},
			"docker-registry.domain.name:5000/repo/image1:ver",
		},
		{
			Image{Registry: "shortreg", Name: "repo/image1", Tag: ""},
			"shortreg/repo/image1",
		},
		{
			Image{Registry: "", Name: "foo", Tag: ""},
			"foo",
		},
	} {
		out := input.in.String()
		if out != input.out {
			t.Fatalf("Image{%q, %q, %q}.String(): %s != %s", input.in, out, input.out)
		}
	}
}

func TestImageRepository(t *testing.T) {
	for _, input := range []struct {
		in  Image
		out string
	}{
		{
			Image{Registry: "", Name: "foo/bar", Tag: ""},
			"foo/bar",
		},
		{
			Image{Registry: "", Name: "foo/bar", Tag: "baz"},
			"foo/bar",
		},
		{
			Image{Registry: "reg:123", Name: "foo/bar", Tag: "baz"},
			"reg:123/foo/bar",
		},
		{
			Image{Registry: "docker-registry.domain.name:5000", Name: "repo/image1", Tag: "ver"},
			"docker-registry.domain.name:5000/repo/image1",
		},
		{
			Image{Registry: "shortreg", Name: "repo/image1", Tag: ""},
			"shortreg/repo/image1",
		},
		{
			Image{Registry: "", Name: "foo", Tag: ""},
			"foo",
		},
	} {
		out := input.in.Repository()
		if out != input.out {
			t.Fatalf("Image{%q, %q, %q}.Repository(): %s != %s", input.in, out, input.out)
		}
	}
}
