package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHelmValues(t *testing.T) {
	testCases := []struct {
		original         *HelmValues
		transformer      func(v *HelmValues) *HelmValues
		expectedCopy     *HelmValues
		expectedOriginal *HelmValues
	}{
		// reassignment
		{
			original: nil,
			transformer: func(v *HelmValues) *HelmValues {
				return &HelmValues{}
			},
			expectedCopy:     &HelmValues{},
			expectedOriginal: nil,
		},
		// mutation
		{
			original: &HelmValues{Values: map[string]interface{}{}},
			transformer: func(v *HelmValues) *HelmValues {
				v.Values["foo"] = "bar"
				return v
			},
			expectedCopy:     &HelmValues{Values: map[string]interface{}{"foo": "bar"}},
			expectedOriginal: &HelmValues{Values: map[string]interface{}{}},
		},
		{
			original: &HelmValues{Values: map[string]interface{}{"foo": map[string]interface{}{"bar": "baz"}}},
			transformer: func(v *HelmValues) *HelmValues {
				v.Values["foo"] = map[string]interface{}{"bar": "oof"}
				return v
			},
			expectedCopy:     &HelmValues{Values: map[string]interface{}{"foo": map[string]interface{}{"bar": "oof"}}},
			expectedOriginal: &HelmValues{Values: map[string]interface{}{"foo": map[string]interface{}{"bar": "baz"}}},
		},
	}

	for i, tc := range testCases {
		output := &HelmValues{}
		tc.original.DeepCopyInto(output)
		assert.Exactly(t, tc.expectedCopy, tc.transformer(output), "copy was not mutated. test case: %d", i)
		assert.Exactly(t, tc.expectedOriginal, tc.original, "original was mutated. test case: %d", i)
	}
}
