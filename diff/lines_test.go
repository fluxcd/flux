package diff

import (
	"reflect"
	"testing"
)

func testLineDiff(t *testing.T, a, b []string, expected []Difference) {
	diffs, err := DiffLines(a, b, "lines")
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(expected, diffs) {
		t.Errorf("expected:\n%#v\ngot:\n%#v", expected, diffs)
	}
}

func TestEmptyLineDiff(t *testing.T) {
	testLineDiff(t, nil, nil, nil)
	testLineDiff(t, nil, []string{}, nil)
	testLineDiff(t, []string{}, nil, nil)
}

func TestSomeVsNoneLinesDiff(t *testing.T) {
	expected := []Difference{
		Added("added", "lines[0]"),
	}
	testLineDiff(t, nil, []string{"added"}, expected)
	testLineDiff(t, []string{}, []string{"added"}, expected)
}

func TestSingleLineAdd(t *testing.T) {
	a := []string{"foo", "bar", "baz"}
	b := []string{"foo", "bar", "boom"}
	expected := []Difference{
		Changed("baz", "boom", "lines[2]"),
	}
	testLineDiff(t, a, b, expected)
}

func TestMultipleLineDiff(t *testing.T) {
	a := []string{"one", "two", "three", "four", "five"}
	b := []string{"one", "2", "three", "4", "five"}
	expected := []Difference{
		Changed("two", "2", "lines[1]"),
		Changed("four", "4", "lines[3]"),
	}
	testLineDiff(t, a, b, expected)
}
