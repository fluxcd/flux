package middleware

import (
	"net/http"
	"regexp"
	"strings"
)

// Workaround for quay.io, which fails to quote the scope value in its
// WWW-Authenticate header. Annoying.  This also remembers a Bearer
// token, so once you have authenticated, it can keep using it rather
// than authenticating each time.

type WWWAuthenticateFixer struct {
	Transport   http.RoundTripper
	tokenHeader string
}

func (t *WWWAuthenticateFixer) RoundTrip(req *http.Request) (*http.Response, error) {
	t.maybeAddToken(req)
	res, err := t.Transport.RoundTrip(req)
	if err == nil {
		newAuthHeaders := []string{}
		for _, h := range res.Header[http.CanonicalHeaderKey("WWW-Authenticate")] {
			if strings.HasPrefix(h, "Bearer ") {
				h = replaceUnquoted(h)
			}
			newAuthHeaders = append(newAuthHeaders, h)
		}
		res.Header[http.CanonicalHeaderKey("WWW-Authenticate")] = newAuthHeaders
	}
	return res, err
}

var scopeRE *regexp.Regexp = regexp.MustCompile(`,scope=([^"].*[^"])$`)

// This is pretty specific. quay.io leaves the `scope` parameter
// unquoted, which trips up parsers (the one in the library we're
// using, for example). So replace an unquoted value with a quoted
// value, for that parameter.
func replaceUnquoted(h string) string {
	return scopeRE.ReplaceAllString(h, `,scope="$1"`)
}

// If we've got a token from a previous roundtrip, try using it
// again. BEWARE: this means this transport should only be used when
// asking (repeatedly) about a single repository, otherwise we may
// leak authorisation.
func (t *WWWAuthenticateFixer) maybeAddToken(req *http.Request) {
	authHeaders := req.Header[http.CanonicalHeaderKey("Authorization")]
	for _, h := range authHeaders {
		if strings.EqualFold(h[:7], "bearer ") {
			if t.tokenHeader == "" {
				t.tokenHeader = h
			}
			return
		}
	}
	if t.tokenHeader != "" {
		req.Header.Set("Authorization", t.tokenHeader)
	}
}
