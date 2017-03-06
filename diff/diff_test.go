package diff

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// --- test diffing objects

type base struct {
	Kind, Namespace, Name string
}

func (b base) ID() ObjectID {
	return ObjectID{b.Kind, b.Namespace, b.Name}
}

func (b base) Source() string {
	return "fake"
}

type TestValue struct {
	base
	ignoreUnexported string
	StringField      string
	IntField         int
	Differ           TestDiffer
	DifferStar       *TestDiffer
	Embedded         struct {
		NestedValue bool
	}
}

type TestDiffer struct {
	CaseInsensitive string
}

func (t TestDiffer) Diff(d Differ, path string) ([]Difference, error) {
	var other *TestDiffer
	switch d := d.(type) {
	case TestDiffer:
		other = &d
	case *TestDiffer:
		other = d
	default:
		return nil, errors.New("not diffable values")
	}

	if !strings.EqualFold(t.CaseInsensitive, other.CaseInsensitive) {
		return []Difference{Changed{t.CaseInsensitive, other.CaseInsensitive, path}}, nil
	}
	return nil, nil
}

func TestFieldwiseDiff(t *testing.T) {
	id := base{"TestFieldwise", "namespace", "testcase"}

	a := TestValue{
		base:             id,
		ignoreUnexported: "one value",
		StringField:      "ground value",
		IntField:         5,
		Differ:           TestDiffer{"case-insensitive"},
		DifferStar:       &TestDiffer{"case-insensitive"},
	}
	a.Embedded.NestedValue = true

	b := TestValue{
		base:             id,
		ignoreUnexported: "completely different value",
		StringField:      "a different ground value",
		IntField:         7,
		Differ:           TestDiffer{"CASE-INSENSITIVE"},
		DifferStar:       &TestDiffer{"CASE-INSENSITIVE"},
	}
	b.Embedded.NestedValue = false

	diffs, err := DiffObject(a, a)
	if err != nil {
		t.Error(err)
	}
	if len(diffs) > 0 {
		t.Errorf("expected no diffs, got %#v", diffs)
	}

	diffs, err = DiffObject(a, b)
	if err != nil {
		t.Error(err)
	}
	if len(diffs) != 3 {
		t.Errorf("expected three diffs, got:\n%#v", diffs)
	}
}

// --- test whole `ObjectSet`s

func TestEmptyVsEmpty(t *testing.T) {
	setA := MakeObjectSet("A")
	setB := MakeObjectSet("B")
	diff, err := DiffSet(setA, setB)
	if err != nil {
		t.Error(err)
	}
	if len(diff.OnlyA) > 0 || len(diff.OnlyB) > 0 || len(diff.Different) > 0 {
		t.Errorf("expected no differences, got %#v", diff)
	}
}

func TestSomeVsNone(t *testing.T) {
	objA := base{"Deployment", "a-namespace", "a-name"}

	setA := MakeObjectSet("A")
	setA.Objects[objA.ID()] = objA
	setB := MakeObjectSet("B")

	diff, err := DiffSet(setA, setB)
	if err != nil {
		t.Error(err)
	}

	expected := MakeObjectSetDiff(setA, setB)
	expected.OnlyA = []Object{objA}
	if !reflect.DeepEqual(expected, diff) {
		t.Errorf("expected:\n%#v\ngot:\n%#v", expected, diff)
	}
}

func TestNoneVsSome(t *testing.T) {
	objB := base{"Deployment", "b-namespace", "b-name"}

	setA := MakeObjectSet("A")
	setB := MakeObjectSet("B")
	setB.Objects[objB.ID()] = objB

	diff, err := DiffSet(setA, setB)
	if err != nil {
		t.Error(err)
	}

	expected := MakeObjectSetDiff(setA, setB)
	expected.OnlyB = []Object{objB}
	if !reflect.DeepEqual(expected, diff) {
		t.Errorf("expected:\n%#v\ngot:\n%#v", expected, diff)
	}
}

func TestSliceDiff(t *testing.T) {
	a := []string{"a", "b", "c"}
	b := []string{"a", "b'"}
	diffs, err := diffObj(reflect.ValueOf(a), reflect.ValueOf(b), reflect.TypeOf(a), "slice")
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) == 0 {
		t.Fatal("expected more than zero differences, but got zero")
	}

	expected := []Difference{
		Changed{"b", "b'", "slice[1]"},
		Removed{"c", "slice[2]"},
	}
	if !reflect.DeepEqual(expected, diffs) {
		t.Errorf("expected diff:\n%#v\ngot:\n%#v\n", expected, diffs)
	}
}

func TestMapDiff(t *testing.T) {
	a := map[string]string{
		"one":   "foo",
		"two":   "bar",
		"three": "baz",
	}
	b := map[string]string{
		"one":  "foo",
		"two":  "bart",
		"four": "shamu",
	}

	diffs, err := diffObj(reflect.ValueOf(a), reflect.ValueOf(b), reflect.TypeOf(a), "map")
	if err != nil {
		t.Fatal(err)
	}

	expected := []Difference{
		Removed{"baz", "map[three]"},
		Changed{"bar", "bart", "map[two]"},
		Added{"shamu", "map[four]"},
	}
	if !reflect.DeepEqual(expected, diffs) {
		t.Errorf("expected diff:\n%#v\ngot:\n%#v\n", expected, diffs)
	}
}
