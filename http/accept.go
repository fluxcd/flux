package http

import (
	"net/http"
	"sort"

	"github.com/golang/gddo/httputil/header"
)

// negotiateContentType picks a content type based on the Accept
// header from a request, and a supplied list of available content
// types in order of preference. If the Accept header mentions more
// than one available content type, the one with the highest quality
// (`q`) parameter is chosen; if there are a number of those, the one
// that appears first in the available types is chosen.
func negotiateContentType(r *http.Request, orderedPref []string) string {
	specs := header.ParseAccept(r.Header, "Accept")
	if len(specs) == 0 {
		return orderedPref[0]
	}

	preferred := []header.AcceptSpec{}
	for _, spec := range specs {
		if indexOf(orderedPref, spec.Value) < len(orderedPref) {
			preferred = append(preferred, spec)
		}
	}
	if len(preferred) > 0 {
		sort.Sort(SortAccept{preferred, orderedPref})
		return preferred[0].Value
	}
	return ""
}

type SortAccept struct {
	specs []header.AcceptSpec
	prefs []string
}

func (s SortAccept) Len() int {
	return len(s.specs)
}

// We want to sort by descending order of suitability: higher quality
// to lower quality, and preferred to less preferred.
func (s SortAccept) Less(i, j int) bool {
	switch {
	case s.specs[i].Q == s.specs[j].Q:
		return indexOf(s.prefs, s.specs[i].Value) < indexOf(s.prefs, s.specs[j].Value)
	default:
		return s.specs[i].Q > s.specs[j].Q
	}
}

func (s SortAccept) Swap(i, j int) {
	s.specs[i], s.specs[j] = s.specs[j], s.specs[i]
}

// This exists so we can search short slices of strings without
// requiring them to be sorted. Returning the len value if not found
// is so that it can be used directly in a comparison when sorting (a
// `-1` would mean "not found" was sorted before found entries).
func indexOf(ss []string, search string) int {
	for i, s := range ss {
		if s == search {
			return i
		}
	}
	return len(ss)
}
