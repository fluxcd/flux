package policy

import (
	"errors"
	"regexp"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/ryanuber/go-glob"
	"github.com/weaveworks/flux/image"
)

const (
	globPrefix       = "glob:"
	semverPrefix     = "semver:"
	globSemverPrefix = "globsemver:"
	regexpPrefix     = "regexp:"
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
	pattern string // pattern without prefix
	regexp  *regexp.Regexp
}

// GlobSemverPattern matches by glob and respects semantic versioning.
type GlobSemverPattern struct {
	pattern     string // pattern without prefix
	prefix      string
	ver         string
	suffix      string
	extractErr  error
	constraints *semver.Constraints
}

// splits "v{1.2.3}-dev" into  "v", "1.2.3" and "-dev"
func splitSemVer(pattern string) (string, string, string, error) {
	x := strings.Split(pattern, "{")
	if len(x) != 2 {
		return "", "", "", errors.New("invalid format for GlobSemver tag")
	}
	prefix, remaining := x[0], x[1]
	y := strings.Split(remaining, "}")
	if len(y) != 2 {
		return "", "", "", errors.New("invalid format for GlobSemver tag")
	}
	ver, suffix := y[0], y[1]
	if len(ver) == 0 {
		return "", "", "", errors.New("invalid format for GlobSemver tag, empty semver pattern")
	}
	return prefix, ver, suffix, nil
}

// NewPattern instantiates a Pattern according to the prefix
// it finds. The prefix can be either `glob:` (default if omitted),
// `semver:`, `globsemver:` or `regexp:`.
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
	case strings.HasPrefix(pattern, globSemverPrefix):
		pattern = strings.TrimPrefix(pattern, globSemverPrefix)
		prefix, ver, suffix, err := splitSemVer(pattern)
		c, _ := semver.NewConstraint(ver)
		return GlobSemverPattern{pattern, prefix, ver, suffix, err, c}
	default:
		return GlobPattern(strings.TrimPrefix(pattern, globPrefix))
	}
}

func (gs GlobSemverPattern) ExtractSemver(tag string) string {
	tagSemver := strings.TrimSuffix(tag, gs.suffix)
	return strings.TrimPrefix(tagSemver, gs.prefix)
}

func (gs GlobSemverPattern) Matches(tag string) bool {
	v, err := semver.NewVersion(gs.ExtractSemver(tag))
	if err != nil {
		return false
	}
	var semverMatch bool
	if gs.constraints == nil {
		// Invalid constraints match anything
		semverMatch = true
	} else {
		semverMatch = gs.constraints.Check(v)
	}
	globPattern := gs.prefix + "*" + gs.suffix
	g := glob.Glob(globPattern, tag)
	return g && semverMatch
}

func (gs GlobSemverPattern) Newer(a, b *image.Info) bool {
	oldATag, oldBTag := a.ID.Tag, b.ID.Tag
	a.ID.Tag, b.ID.Tag = gs.ExtractSemver(a.ID.Tag), gs.ExtractSemver(b.ID.Tag)
	isNever := image.NewerBySemver(a, b)
	a.ID.Tag, b.ID.Tag = oldATag, oldBTag
	return isNever
}

func (gs GlobSemverPattern) String() string {
	return globSemverPrefix + gs.pattern
}

func (gs GlobSemverPattern) Valid() bool {
	return gs.constraints != nil && gs.extractErr == nil
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
