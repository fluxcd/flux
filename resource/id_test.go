package resource

import (
	"testing"
)

func TestResourceIDParsing(t *testing.T) {
	type test struct {
		name, id string
	}
	valid := []test{
		{"full", "namespace:kind/name"},
		{"legacy", "namespace/service"},
		{"dots", "namespace:kind/name.with.dots"},
		{"colons", "namespace:kind/name:with:colons"},
		{"punctuation in general", "name-space:ki_nd/punc_tu:a.tion-rules"},
		{"cluster-scope resource", "<cluster>:namespace/foo"},
	}
	invalid := []test{
		{"unqualified", "justname"},
		{"dots in namespace", "name.space:kind/name"},
		{"too many colons", "namespace:kind:name"},
	}

	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseID(tc.id); err != nil {
				t.Error(err)
			}
		})
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseID(tc.id); err == nil {
				t.Errorf("expected %q to be considered invalid", tc.id)
			}
		})
	}
}
