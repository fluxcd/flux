package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIncluderFunc(t *testing.T) {
	in := IncluderFunc(func(s string) bool {
		return s == "included"
	})
	assert.True(t, in.IsIncluded("included"))
	assert.False(t, in.IsIncluded("excluded"))
}

func TestExcludeInclude(t *testing.T) {
	test := func(ei Includer, s string, expected bool) {
		if expected {
			t.Run("includes "+s, func(t *testing.T) {
				assert.True(t, ei.IsIncluded(s))
			})
		} else {
			t.Run("excludes "+s, func(t *testing.T) {
				assert.False(t, ei.IsIncluded(s))
			})
		}
	}

	// Only exclude stuff
	ei1 := ExcludeIncludeGlob{
		Exclude: []string{"foo/*"},
	}

	for _, t := range []string{
		"",
		"completely unrelated",
		"foo",
		"starts/foo/bar",
	} {
		test(ei1, t, true)
	}

	for _, t := range []string{
		"foo/bar",
	} {
		test(ei1, t, false)
	}

	// Explicitly include stuff
	ei2 := ExcludeIncludeGlob{
		Exclude: []string{"foo/bar/*"},
		Include: []string{"foo/*", "boo/*"},
	}

	for _, t := range []string{
		"boo/whatever",
		"foo/something/else",
	} {
		test(ei2, t, true)
	}

	for _, t := range []string{
		"baz/anything",
		"foo/bar/something",
		"anything not explicitly included",
	} {
		test(ei2, t, false)
	}
}
