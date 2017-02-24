package git

import (
	"testing"
)

func TestParseURL(t *testing.T) {
	for _, example := range []struct {
		url, host, owner, repo string
		err                    error
	}{
		{"", "", "", "", ErrInvalidGitURL},
		{"git@github.com:weaveworks", "", "", "", ErrInvalidGitURL},
		{"git@github.com:weaveworks/flux", "github.com", "weaveworks", "flux", nil},
		{"https://github.com/weaveworks", "", "", "", ErrInvalidGitURL},
		{"https://github.com/weaveworks/flux.git", "github.com", "weaveworks", "flux", nil},
		{"https://github.com/weaveworks/flux", "github.com", "weaveworks", "flux", nil},
		{"https://github.com/weaveworks", "", "", "", ErrInvalidGitURL},
	} {
		host, owner, repo, err := ParseURL(example.url)
		if err != example.err {
			t.Errorf("[%s] Expected err: %v, Got %v", example.url, example.err, err)
			continue
		}
		if host != example.host {
			t.Errorf("[%s] Expected host: %q, Got %q", example.url, example.host, host)
		}
		if owner != example.owner {
			t.Errorf("[%s] Expected owner: %q, Got %q", example.url, example.owner, owner)
		}
		if repo != example.repo {
			t.Errorf("[%s] Expected repo: %q, Got %q", example.url, example.repo, repo)
		}
	}
}
