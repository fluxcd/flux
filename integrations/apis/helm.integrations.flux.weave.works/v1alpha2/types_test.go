package v1alpha2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFluxHelmValues(t *testing.T) {
	testCases := []struct {
		original         *FluxHelmValues
		transformer      func(v *FluxHelmValues) *FluxHelmValues
		expectedCopy     *FluxHelmValues
		expectedOriginal *FluxHelmValues
	}{
		// reassignment
		{
			original: nil,
			transformer: func(v *FluxHelmValues) *FluxHelmValues {
				return &FluxHelmValues{}
			},
			expectedCopy:     &FluxHelmValues{},
			expectedOriginal: nil,
		},
		// mutation
		{
			original: &FluxHelmValues{Values: map[string]interface{}{}},
			transformer: func(v *FluxHelmValues) *FluxHelmValues {
				v.Values["foo"] = "bar"
				return v
			},
			expectedCopy:     &FluxHelmValues{Values: map[string]interface{}{"foo": "bar"}},
			expectedOriginal: &FluxHelmValues{Values: map[string]interface{}{}},
		},
		{
			original: &FluxHelmValues{Values: map[string]interface{}{"foo": map[string]interface{}{"bar": "baz"}}},
			transformer: func(v *FluxHelmValues) *FluxHelmValues {
				v.Values["foo"] = map[string]interface{}{"bar": "oof"}
				return v
			},
			expectedCopy:     &FluxHelmValues{Values: map[string]interface{}{"foo": map[string]interface{}{"bar": "oof"}}},
			expectedOriginal: &FluxHelmValues{Values: map[string]interface{}{"foo": map[string]interface{}{"bar": "baz"}}},
		},
	}

	for i, tc := range testCases {
		output := &FluxHelmValues{}
		tc.original.DeepCopyInto(output)
		assert.Exactly(t, tc.expectedCopy, tc.transformer(output), "copy was not mutated. test case: %d", i)
		assert.Exactly(t, tc.expectedOriginal, tc.original, "original was mutated. test case: %d", i)
	}
}
