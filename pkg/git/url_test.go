package git

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestEquivalent(t *testing.T) {
	urls := []struct {
		remote     string
		equivalent string
		equal      bool
	}{
		{"git@github.com:fluxcd/flux", "ssh://git@github.com/fluxcd/flux.git", true},
		{"https://git@github.com/fluxcd/flux.git", "ssh://git@github.com/fluxcd/flux.git", true},
		{"https://github.com/fluxcd/flux.git", "git@github.com:fluxcd/flux.git", true},
		{"https://github.com/fluxcd/flux.git", "https://github.com/fluxcd/helm-operator.git", false},
	}

	for _, u := range urls {
		r := Remote{u.remote}
		assert.Equal(t, u.equal, r.Equivalent(u.equivalent))
	}
}

func TestHasValidHostname(t *testing.T) {
	for _, url := range []string{
		"git@github.com:fluxcd/flux",
		"https://user@example.com:5050/repo.git",
	} {
		u := Remote{url}
		if hostname, isValid := u.HasValidHostname(); !isValid {
			t.Errorf("git repo URL %s contains invalid hostname %s", url, hostname)
		}
	}
}
