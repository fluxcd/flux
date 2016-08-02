package registry

import "testing"

var imageParsingExamples = map[string]Image{
	"foo/bar": Image{
		Registry: "",
		Name:     "foo/bar",
		Tag:      "",
	},
	"foo/bar:baz": Image{
		Registry: "",
		Name:     "foo/bar",
		Tag:      "baz",
	},
	"reg:123/foo/bar:baz": Image{
		Registry: "reg:123",
		Name:     "foo/bar",
		Tag:      "baz",
	},
	"docker-registry.domain.name:5000/repo/image1:ver": Image{
		Registry: "docker-registry.domain.name:5000",
		Name:     "repo/image1",
		Tag:      "ver",
	},
	"shortreg/repo/image1": Image{
		Registry: "shortreg",
		Name:     "repo/image1",
		Tag:      "",
	},
	"foo": Image{
		Registry: "",
		Name:     "foo",
		Tag:      "",
	},
}

func TestParseImage(t *testing.T) {
	for in, want := range imageParsingExamples {
		out := ParseImage(in)
		if out != want {
			t.Fatalf("%s: %v != %v", in, out, want)
		}
	}
}

func TestImageString(t *testing.T) {
	for want, in := range imageParsingExamples {
		out := in.String()
		if out != want {
			t.Fatalf("%#v.String(): %s != %s", in, out, want)
		}
	}
}

func TestImageRepository(t *testing.T) {
	for in, want := range map[Image]string{
		Image{Name: "foo/bar"}:                                                               "foo/bar",
		Image{Name: "foo/bar", Tag: "baz"}:                                                   "foo/bar",
		Image{Registry: "reg:123", Name: "foo/bar", Tag: "baz"}:                              "reg:123/foo/bar",
		Image{Registry: "docker-registry.domain.name:5000", Name: "repo/image1", Tag: "ver"}: "docker-registry.domain.name:5000/repo/image1",
		Image{Registry: "shortreg", Name: "repo/image1"}:                                     "shortreg/repo/image1",
		Image{Name: "foo"}:                                                                   "foo",
	} {
		out := in.Repository()
		if out != want {
			t.Fatalf("%#v.Repository(): %s != %s", in, out, want)
		}
	}
}
