package errors

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestZeroErrorEncoding(t *testing.T) {
	type S struct {
		Err *Error
	}
	var s S
	bytes, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var s1 S
	err = json.Unmarshal(bytes, &s1)
	if err != nil {
		t.Fatal(err)
	}
	if s1.Err != nil {
		t.Errorf("expected nil in field, but got %+v", s1.Err)
	}
}

func TestErrorEncoding(t *testing.T) {
	errVal := &Error{
		Type: Server,
		Help: "helpful text\nwith linebreaks!",
		Err:  errors.New("underlying error"),
	}
	bytes, err := json.Marshal(errVal)
	if err != nil {
		t.Fatal(err)
	}

	var got Error
	err = json.Unmarshal(bytes, &got)
	if err != nil {
		t.Fatal(err)
	}

	if got.Type != errVal.Type {
		println(string(bytes))
		t.Errorf("error type: expected %q, got %q", errVal.Type, got.Type)
	}
	if got.Help != errVal.Help || got.Err.Error() != errVal.Err.Error() {
		t.Errorf("expected %+v\ngot %+v", errVal, got)
	}
	if !reflect.DeepEqual(errVal, &got) {
		t.Errorf("not deepEqual\nexpected %#v\ngot %#v", errVal, got)
	}
}
