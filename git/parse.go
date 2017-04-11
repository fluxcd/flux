package git

import (
	"fmt"
	"net/url"
	"strings"
)

var (
	ErrInvalidGitURL = fmt.Errorf("invalid git URL")
)

func ParseURL(u string) (host, owner, repo string, err error) {
	if u == "" {
		return "", "", "", ErrInvalidGitURL
	}

	// Parse shorter scp-like syntax
	if !strings.Contains(u, "://") {
		_, rest := pop(u, "@")
		host, path := pop(rest, ":")
		owner, repo = pop(path, "/")
		if host == "" || owner == "" || repo == "" {
			return "", "", "", ErrInvalidGitURL
		}
		return host, owner, strings.TrimSuffix(repo, ".git"), nil
	}

	// Try to parse it as an http-style url
	parsed, err := url.Parse(u)
	if err == nil && parsed.Scheme != "" {
		owner, repo = pop(strings.TrimPrefix(parsed.Path, "/"), "/")
		if owner == "" || repo == "" {
			return "", "", "", ErrInvalidGitURL
		}
		return parsed.Host, owner, strings.TrimSuffix(repo, ".git"), nil
	}

	return "", "", "", ErrInvalidGitURL
}

// pop splits a string in 2 at sep, returning everything before and after. if
// the string doesn't contain sep, then tail will be ""
func pop(s, sep string) (head, tail string) {
	parts := strings.SplitN(s, sep, 2)
	head = parts[0]
	if len(parts) > 1 {
		tail = parts[1]
	}
	return
}
