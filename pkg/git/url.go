package git

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/whilp/git-urls"
)

// Remote points at a git repo somewhere.
type Remote struct {
	// URL is where we clone from
	URL string `json:"url"`
}

func (r Remote) SafeURL() string {
	u, err := giturls.Parse(r.URL)
	if err != nil {
		return fmt.Sprintf("<unparseable: %s>", r.URL)
	}
	if u.User != nil {
		u.User = url.User(u.User.Username())
	}
	return u.String()
}

// Equivalent compares the given URL with the remote URL without taking
// protocols or `.git` suffixes into account.
func (r Remote) Equivalent(u string) bool {
	lu, err := giturls.Parse(r.URL)
	if err != nil {
		return false
	}
	ru, err := giturls.Parse(u)
	if err != nil {
		return false
	}
	trimPath := func(p string) string {
		return strings.TrimSuffix(strings.TrimPrefix(p, "/"), ".git")
	}
	return lu.Host == ru.Host && trimPath(lu.Path) == trimPath(ru.Path)
}
