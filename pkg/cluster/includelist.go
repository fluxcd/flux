package cluster

import (
	"github.com/ryanuber/go-glob"
)

// This is to represent "include-exclude" predicate, which is used
// for deciding images to scan.

type Includer interface {
	IsIncluded(string) bool
}

type IncluderFunc func(string) bool

func (f IncluderFunc) IsIncluded(s string) bool {
	return f(s)
}

var AlwaysInclude = IncluderFunc(func(string) bool { return true })

// ExcludeIncludeGlob is an Includer that uses glob patterns to decide
// what to include or exclude. Note that Include and Exclude are
// treated differently -- see the method IsIncluded.
type ExcludeIncludeGlob struct {
	Include []string
	Exclude []string
}

// IsIncluded implements Includer using the logic:
//  - if the string matches any exclude pattern, don't include it
//  - otherwise, if there are no include patterns, include it
//  - otherwise, if it matches an include pattern, include it
//  = otherwise don't include it.
func (ei ExcludeIncludeGlob) IsIncluded(s string) bool {
	for _, ex := range ei.Exclude {
		if glob.Glob(ex, s) {
			return false
		}
	}
	if len(ei.Include) == 0 {
		return true
	}
	for _, in := range ei.Include {
		if glob.Glob(in, s) {
			return true
		}
	}
	return false
}
