package registry

import "testing"

func TestImageParts(t *testing.T) {
	for _, input := range []struct {
		in, repo, name, tag string
	}{
		{"foo/bar", "", "foo/bar", ""},
		{"foo/bar:baz", "", "foo/bar", "baz"},
		{"reg:123/foo/bar:baz", "reg:123", "foo/bar", "baz"},
		{"docker-registry.domain.name:5000/repo/image1:ver", "docker-registry.domain.name:5000", "repo/image1", "ver"},
		{"shortreg/repo/image1", "shortreg", "repo/image1", ""},
		{"foo", "", "foo", ""},
	} {
		repo, name, tag := ImageParts(input.in)
		if repo != input.repo {
			t.Fatalf("%s: %s != %s", input.in, repo, input.repo)
		}
		if name != input.name {
			t.Fatalf("%s: %s != %s", input.in, name, input.name)
		}
		if tag != input.tag {
			t.Fatalf("%s: %s != %s", input.in, tag, input.tag)
		}
	}
}

func TestImageFromParts(t *testing.T) {
	for _, input := range []struct {
		repo, name, tag, out string
	}{
		{"", "foo/bar", "", "foo/bar"},
		{"", "foo/bar", "baz", "foo/bar:baz"},
		{"reg:123", "foo/bar", "baz", "reg:123/foo/bar:baz"},
		{"docker-registry.domain.name:5000", "repo/image1", "ver", "docker-registry.domain.name:5000/repo/image1:ver"},
		{"shortreg", "repo/image1", "", "shortreg/repo/image1"},
		{"", "foo", "", "foo"},
	} {
		out := ImageFromParts(input.repo, input.name, input.tag)
		if out != input.out {
			t.Fatalf("ImageFromParts(%q, %q, %q): %s != %s", input.repo, input.name, input.tag, out, input.out)
		}
	}
}
