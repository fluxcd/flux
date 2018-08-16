package policy

import (
	"github.com/Masterminds/semver"
	"github.com/ryanuber/go-glob"
	"github.com/weaveworks/flux/image"
	"strings"
	"regexp"
)

const (
	globPrefix   = "glob:"
	semverPrefix = "semver:"
	regexpPrefix = "regexp:"
)

var (
	// PatternAll matches everything.
	PatternAll    = NewPattern(globPrefix + "*")
	PatternLatest = NewPattern(globPrefix + "latest")
)

// Pattern provides an interface to match image tags.
type Pattern interface {
	// Matches returns true if the given image tag matches the pattern.
	Matches(tag string) bool
	// String returns the prefixed string representation.
	String() string
	// Newer returns true if image `a` is newer than image `b`.
	Newer(a, b *image.Info) bool
	// Valid returns true if the pattern is considered valid.
	Valid() bool
}

type GlobPattern string

// SemverPattern matches by semantic versioning.
// See https://semver.org/
type SemverPattern struct {
	pattern     string // pattern without prefix
	constraints *semver.Constraints
}

// RegexpPattern matches by regular expression.
type RegexpPattern struct {
	pattern	string // pattern without prefix
	regexp	*regexp.Regexp
}

// NewPattern instantiates a Pattern according to the prefix
// it finds. The prefix can be either `glob:` (default if omitted),
// `semver:` or `regexp:`.
func NewPattern(pattern string) Pattern {
	switch {
	case strings.HasPrefix(pattern, semverPrefix):
		pattern = strings.TrimPrefix(pattern, semverPrefix)
		c, _ := semver.NewConstraint(pattern)
		return SemverPattern{pattern, c}
	case strings.HasPrefix(pattern, regexpPrefix):
		pattern = strings.TrimPrefix(pattern, regexpPrefix)
		r, _ := regexp.Compile(pattern)
		return RegexpPattern{pattern, r}
	default:
		return GlobPattern(strings.TrimPrefix(pattern, globPrefix))
	}
}

func (g GlobPattern) Matches(tag string) bool {
	return glob.Glob(string(g), tag)
}

func (g GlobPattern) String() string {
	return globPrefix + string(g)
}

func (g GlobPattern) Newer(a, b *image.Info) bool {
	return image.NewerByCreated(a, b)
}

func (g GlobPattern) Valid() bool {
	return true
}

func (s SemverPattern) Matches(tag string) bool {
	v, err := semver.NewVersion(tag)
	if err != nil {
		return false
	}
	if s.constraints == nil {
		// Invalid constraints match anything
		return true
	}
	return s.constraints.Check(v)
}

func (s SemverPattern) String() string {
	return semverPrefix + s.pattern
}

func (s SemverPattern) Newer(a, b *image.Info) bool {
	return image.NewerBySemver(a, b)
}

func (s SemverPattern) Valid() bool {
	return s.constraints != nil
}

func (r RegexpPattern) Matches(tag string) bool {
	if r.regexp == nil {
		// Invalid regexp match anything
		return true
	}
	return r.regexp.MatchString(tag)
}

func (r RegexpPattern) String() string {
	return regexpPrefix + r.pattern
}

func (r RegexpPattern) Newer(a, b *image.Info) bool {
	return image.NewerByCreated(a, b)
}

func (r RegexpPattern) Valid() bool {
	return r.regexp != nil
}
