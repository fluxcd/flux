package resource

import (
	"github.com/weaveworks/flux/diff"
)

type Secret struct {
	baseObject
	Data map[string]SecretData
	Type string
}

type SecretData string

func (s SecretData) Diff(d diff.Differ, path string) ([]diff.Difference, error) {
	if s1, ok := d.(SecretData); ok {
		if s1 == s {
			return nil, nil
		}
		return []diff.Difference{diff.OpaqueChanged{path}}, nil
	}
	return nil, diff.ErrNotDiffable
}
