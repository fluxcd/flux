package git

import (
	"fmt"
	"net/url"

	"github.com/whilp/git-urls"
)

// Remote points at a git repo somewhere.
type Remote struct {
	URL string // clone from here
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
