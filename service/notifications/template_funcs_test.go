package notifications

import (
	"errors"
	"testing"

	"github.com/weaveworks/flux/update"
)

func TestTemplateFunc_Last(t *testing.T) {
	for _, test := range []struct {
		i           int
		a           interface{}
		expectedVal bool
		expectedErr error
	}{
		// nil slices
		{-1, []string(nil), true, nil},
		{0, []string(nil), false, nil},
		{1, []string(nil), false, nil},

		// empty slices
		{-1, []string{}, true, nil},
		{0, []string{}, false, nil},
		{1, []string{}, false, nil},

		// slices with values
		{-1, []string{"a"}, false, nil},
		{0, []string{"a"}, true, nil},
		{1, []string{"a"}, false, nil},

		// arrays
		{-1, [1]string{"a"}, false, nil},
		{0, [1]string{"a"}, true, nil},
		{1, [1]string{"a"}, false, nil},

		// maps
		{0, map[string]string{"a": "b"}, true, nil},
		{1, map[string]string{"a": "b"}, false, nil},

		// strings
		{0, "ab", false, nil},
		{1, "ab", true, nil},

		// Shouldn't panic
		{0, struct{}{}, false, errors.New("unsupported type: struct {}")},
		{0, nil, false, errors.New("unsupported type: <nil>")},
		{0, update.Spec{}, false, errors.New("unsupported type: update.Spec")},
	} {
		gotVal, gotErr := last(test.i, test.a)
		if gotVal != test.expectedVal {
			t.Errorf("last(%v, %v): (%v, %v) expected (%v, %v)", test.i, test.a, gotVal, gotErr, test.expectedVal, test.expectedErr)
		} else if test.expectedErr != nil && gotErr == nil {
			t.Errorf("last(%v, %v): (%v, %v) expected (%v, %v)", test.i, test.a, gotVal, gotErr, test.expectedVal, test.expectedErr)
		} else if test.expectedErr == nil && gotErr != nil {
			t.Errorf("last(%v, %v): (%v, %v) expected (%v, %v)", test.i, test.a, gotVal, gotErr, test.expectedVal, test.expectedErr)
		} else if test.expectedErr != nil && gotErr != nil && gotErr.Error() != test.expectedErr.Error() {
			t.Errorf("last(%v, %v): (%v, %v) expected (%v, %v)", test.i, test.a, gotVal, gotErr, test.expectedVal, test.expectedErr)
		}
	}
}
