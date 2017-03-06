package diff

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
)

// Difference represents an individual difference between two
// `Object`s. This is an interface because
type Difference interface {
	Summarise(out io.Writer)
}

type Changed struct {
	A, B interface{}
	Path string
}

type Added struct {
	Value interface{}
	Path  string
}

type Removed struct {
	Value interface{}
	Path  string
}

// the value has changed, but don't report the before or after
type OpaqueChanged struct {
	Path string
}

// Objects are considered unique by {Namespace, Kind, Name}
type ObjectID struct {
	Namespace string
	Kind      string
	Name      string
}

type Object interface {
	ID() ObjectID
	Source() string
}

// ObjectSet is a set of several objects which can be diffed
// collectively.
type ObjectSet struct {
	Source  string
	Objects map[ObjectID]Object
}

func MakeObjectSet(source string) *ObjectSet {
	return &ObjectSet{
		Source:  source,
		Objects: map[ObjectID]Object{},
	}
}

type ObjectSetDiff struct {
	A, B      *ObjectSet
	OnlyA     []Object
	OnlyB     []Object
	Different map[ObjectID][]Difference
}

func MakeObjectSetDiff(a, b *ObjectSet) ObjectSetDiff {
	return ObjectSetDiff{
		A:         a,
		B:         b,
		Different: map[ObjectID][]Difference{},
	}
}

// Diff calculates the differences between one model and another
func DiffSet(a, b *ObjectSet) (ObjectSetDiff, error) {
	diff := MakeObjectSetDiff(a, b)

	// A - B and A ^ B at the same time
	for id, objA := range a.Objects {
		if objB, found := b.Objects[id]; found {
			objDiff, err := DiffObject(objA, objB)
			if err != nil {
				return diff, err
			}
			if len(objDiff) > 0 {
				diff.Different[id] = objDiff
			}
		} else {
			diff.OnlyA = append(diff.OnlyA, objA)
		}
	}
	// now, B - A
	for id, objB := range b.Objects {
		if _, found := a.Objects[id]; !found {
			diff.OnlyB = append(diff.OnlyB, objB)
		}
	}
	return diff, nil
}

type Differ interface {
	Diff(a Differ, path string) ([]Difference, error)
}

var ErrNotDiffable = errors.New("values are not diffable")

// Diff one object with another. This assumes that the objects being
// compared are supposed to represent the same logical object, i.e.,
// they were identified with the same ID. An error indicates they are
// not comparable.
func DiffObject(a, b Object) ([]Difference, error) {
	if a.ID() != b.ID() {
		return nil, errors.New("objects being compared do not have the same ID")
	}

	// Special case at the top: if these have different runtime types,
	// they are not comparable.
	typA, typB := reflect.TypeOf(a), reflect.TypeOf(b)
	if typA != typB {
		return nil, ErrNotDiffable
	}
	return diffObj(reflect.ValueOf(a), reflect.ValueOf(b), typA, "")
}

var differType = reflect.TypeOf((*Differ)(nil)).Elem()

// Compare two values and compile a list of differences between them.
func diffObj(a, b reflect.Value, typ reflect.Type, path string) ([]Difference, error) {
	if typ.Implements(differType) {
		differA, differB := a.Interface().(Differ), b.Interface().(Differ)
		return differA.Diff(differB, path)
	}

	switch typ.Kind() {
	case reflect.Array:
		fallthrough
	case reflect.Slice:
		return diffArrayOrSlice(a, b, typ, path)
	case reflect.Interface:
		return nil, errors.New("interface diff not implemented")
	case reflect.Ptr:
		a, b, typ = reflect.Indirect(a), reflect.Indirect(b), typ.Elem()
		return diffObj(a, b, typ, path)
	case reflect.Struct:
		return diffStruct(a, b, typ, path)
	case reflect.Map:
		return diffMap(a, b, typ.Elem(), path)
	case reflect.Func:
		return nil, errors.New("func diff not implemented (and not implementable)")
	default: // all ground types
		if a.Interface() != b.Interface() {
			return []Difference{Changed{a.Interface(), b.Interface(), path}}, nil
		}
		return nil, nil
	}
}

// diff each exported field individually
func diffStruct(a, b reflect.Value, structTyp reflect.Type, path string) ([]Difference, error) {
	var diffs []Difference

	for i := 0; i < structTyp.NumField(); i++ {
		field := structTyp.Field(i)
		if field.PkgPath == "" { // i.e., is an exported field
			fieldDiffs, err := diffObj(a.Field(i), b.Field(i), field.Type, path+"."+field.Name)
			if err != nil {
				return nil, err
			}
			diffs = append(diffs, fieldDiffs...)
		}
	}
	return diffs, nil
}

// diff each element, report over- or underbite
func diffArrayOrSlice(a, b reflect.Value, sliceTyp reflect.Type, path string) ([]Difference, error) {
	var diffs []Difference
	elemTyp := sliceTyp.Elem()

	i := 0
	for ; i < a.Len() && i < b.Len(); i++ {
		d, err := diffObj(a.Index(i), b.Index(i), elemTyp, fmt.Sprintf("%s[%d]", path, i))
		if err != nil {
			return nil, err
		}
		diffs = append(diffs, d...)
	}

	for j := i; j < a.Len(); j++ {
		diffs = append(diffs, Removed{a.Index(j).Interface(), fmt.Sprintf("%s[%d]", path, j)})
	}
	for j := i; j < b.Len(); j++ {
		diffs = append(diffs, Added{b.Index(j).Interface(), fmt.Sprintf("%s[%d]", path, j)})
	}
	return diffs, nil
}

func diffMap(a, b reflect.Value, elemTyp reflect.Type, path string) ([]Difference, error) {
	if a.Kind() != reflect.Map || b.Kind() != reflect.Map {
		return nil, errors.New("both values must be maps")
	}

	var diffs []Difference
	var zero reflect.Value
	for _, keyA := range a.MapKeys() {
		valA := a.MapIndex(keyA)
		if valB := b.MapIndex(keyA); valB != zero {
			moreDiffs, err := diffObj(valA, valB, elemTyp, fmt.Sprintf(`%s[%v]`, path, keyA))
			if err != nil {
				return nil, err
			}
			diffs = append(diffs, moreDiffs...)
		} else {
			diffs = append(diffs, Removed{valA.Interface(), fmt.Sprintf(`%s[%v]`, path, keyA)})
		}
	}
	for _, keyB := range b.MapKeys() {
		valB := b.MapIndex(keyB)
		if valA := a.MapIndex(keyB); valA == zero {
			diffs = append(diffs, Added{valB.Interface(), fmt.Sprintf(`%s[%v]`, path, keyB)})
		}
	}

	sort.Sort(sorted(diffs))
	return diffs, nil
}

// It helps to return the differences for a map in a stable order
type sorted []Difference

func (d sorted) Len() int {
	return len(d)
}

// Sort order for changes: Removed < {Changed, OpaqueChanged} < Added,
// then lexicographic on Path

func (d sorted) Less(i, j int) bool {
	switch a := d[i].(type) {
	case Removed:
		switch b := d[j].(type) {
		case Removed:
			return strings.Compare(a.Path, b.Path) == -1
		default:
			return true
		}
	case Changed:
		switch b := d[j].(type) {
		case Removed:
			return false
		case Changed:
			return strings.Compare(a.Path, b.Path) == -1
		case OpaqueChanged:
			return strings.Compare(a.Path, b.Path) == -1
		default:
			return true
		}
	case OpaqueChanged:
		switch b := d[j].(type) {
		case Removed:
			return false
		case Changed:
			return strings.Compare(a.Path, b.Path) == -1
		case OpaqueChanged:
			return strings.Compare(a.Path, b.Path) == -1
		default:
			return true
		}
	case Added:
		switch b := d[j].(type) {
		case Added:
			return strings.Compare(a.Path, b.Path) == -1
		default:
			return false
		}
	}
	return false
}

func (d sorted) Swap(a, b int) {
	d[a], d[b] = d[b], d[a]
}
