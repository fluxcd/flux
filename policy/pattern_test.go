package policy

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGlobPattern_Matches(t *testing.T) {
	for _, tt := range []struct {
		name    string
		pattern string
		true    []string
		false   []string
	}{
		{
			name:    "all",
			pattern: "*",
			true:    []string{"", "1", "foo"},
			false:   nil,
		},
		{
			name:    "all prefixed",
			pattern: "glob:*",
			true:    []string{"", "1", "foo"},
			false:   nil,
		},
		{
			name:    "prefix",
			pattern: "master-*",
			true:    []string{"master-", "master-foo"},
			false:   []string{"", "foo-master"},
		},
	} {
		pattern := NewPattern(tt.pattern)
		assert.IsType(t, GlobPattern(""), pattern)
		t.Run(tt.name, func(t *testing.T) {
			for _, tag := range tt.true {
				assert.True(t, pattern.Matches(tag))
			}
			for _, tag := range tt.false {
				assert.False(t, pattern.Matches(tag))
			}
		})
	}
}

func TestSemverPattern_Matches(t *testing.T) {
	for _, tt := range []struct {
		name    string
		pattern string
		true    []string
		false   []string
	}{
		{
			name:    "all",
			pattern: "semver:*",
			true:    []string{"1", "1.0", "v1.0.3"},
			false:   []string{"", "latest", "2.0.1-alpha.1"},
		},
		{
			name:    "semver",
			pattern: "semver:~1",
			true:    []string{"v1", "1", "1.2", "1.2.3"},
			false:   []string{"", "latest", "2.0.0"},
		},
		{
			name:    "semver pre-release",
			pattern: "semver:2.0.1-alpha.1",
			true:    []string{"2.0.1-alpha.1"},
			false:   []string{"2.0.1"},
		},
	} {
		pattern := NewPattern(tt.pattern)
		assert.IsType(t, SemverPattern{}, pattern)
		for _, tag := range tt.true {
			t.Run(fmt.Sprintf("%s[%q]", tt.name, tag), func(t *testing.T) {
				assert.True(t, pattern.Matches(tag))
			})
		}
		for _, tag := range tt.false {
			t.Run(fmt.Sprintf("%s[%q]", tt.name, tag), func(t *testing.T) {
				assert.False(t, pattern.Matches(tag))
			})
		}
	}
}

func TestRegexpPattern_Matches(t *testing.T) {
	for _, tt := range []struct {
		name    string
		pattern string
		true    []string
		false   []string
	}{
		{
			name:    "all prefixed",
			pattern: "regexp:(.*?)",
			true:    []string{"", "1", "foo"},
			false:   nil,
		},
		{
			name:    "regexp",
			pattern: "regexp:^([a-zA-Z]+)$",
			true:    []string{"foo", "BAR", "fooBAR"},
			false:   []string{"1", "foo-1"},
		},
	} {
		pattern := NewPattern(tt.pattern)
		assert.IsType(t, RegexpPattern{}, pattern)
		for _, tag := range tt.true {
			t.Run(fmt.Sprintf("%s[%q]", tt.name, tag), func(t *testing.T) {
				assert.True(t, pattern.Matches(tag))
			})
		}
		for _, tag := range tt.false {
			t.Run(fmt.Sprintf("%s[%q]", tt.name, tag), func(t *testing.T) {
				assert.False(t, pattern.Matches(tag))
			})
		}
	}
}

func Test_splitSemVer(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		ver     string
		broken  bool
	}{
		{
			name:    "common case",
			pattern: "v{1.2.3}-stage",
			ver:     "1.2.3",
			broken:  false,
		},
		{
			name:    "missing {",
			pattern: "v1.2.3}-stage",
			broken:  true,
		},
		{
			name:    "missing }",
			pattern: "{1.2.3-stage",
			broken:  true,
		},
		{
			name:    "empty {}",
			pattern: "{}-stage",
			broken:  true,
		},
		{
			name:    "multiple {",
			pattern: "v{{1.2.3}-stage",
			broken:  true,
		},
		{
			name:    "multiple }",
			pattern: "v{1.2.3}}-stage",
			broken:  true,
		},
		{
			name:    "} before {",
			pattern: "v}1.2.3{-stage",
			broken:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2, got3 := splitSemVer(tt.pattern)
			if got1 != tt.ver {
				t.Errorf("extractGlob() got = %v, want %v", got, tt.ver)
			}
			if got3 != nil && !tt.broken {
				t.Errorf("extractGlob() with error %v not marked as broken", got2)
			}
			if got3 == nil && tt.broken {
				t.Errorf("extractGlob() broken pattern should return an error")
			}

		})
	}
}

func TestGlobSemverPattern_Matches(t *testing.T) {
	for _, tt := range []struct {
		name    string
		pattern string
		true    []string
		false   []string
	}{
		{
			name:    "match by version",
			pattern: "globsemver:v{1.2.x}-dev",
			true:    []string{"v1.2.2-dev", "v1.2-dev"},
			false:   []string{"v2.2.2-dev", "v2.2-dev"},
		},
		{
			name:    "match by glob",
			pattern: "globsemver:v{1.2.x}-dev",
			true:    []string{"v1.2.2-dev", "v1.2.9-dev"},
			false:   []string{"v1.2.2-stage", "v1.2.2"},
		},
	} {
		pattern := NewPattern(tt.pattern)
		assert.IsType(t, GlobSemverPattern{}, pattern)
		for _, tag := range tt.true {
			t.Run(fmt.Sprintf("%s[%q]", tt.name, tag), func(t *testing.T) {
				assert.True(t, pattern.Matches(tag))
			})
		}
		for _, tag := range tt.false {
			t.Run(fmt.Sprintf("%s[%q]", tt.name, tag), func(t *testing.T) {
				assert.False(t, pattern.Matches(tag))
			})
		}
	}
}
