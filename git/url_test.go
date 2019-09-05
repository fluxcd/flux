package git

import (
	"strings"
	"testing"
)

func TestSafeURL(t *testing.T) {
	const password = "abc123"
	for _, url := range []string{
		"git@github.com:fluxcd/flux",
		"https://user@example.com:5050/repo.git",
		"https://user:" + password + "@example.com:5050/repo.git",
	} {
		u := Remote{url}
		if strings.Contains(u.SafeURL(), password) {
			t.Errorf("Safe URL for %s contains password %q", url, password)
		}
	}
}
