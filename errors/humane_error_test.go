package errors

import (
	"fmt"
	"github.com/pkg/errors"
	"net/http"
	"testing"
)

func TestHumaneError_ErrorInterface(t *testing.T) {
	err := longError()
	fmt.Println(err.Error())
	if err.Error() == "" {
		t.Fatal("Should not compile, let alone fail.")
	}
}

func TestHumaneError_RootCause(t *testing.T) {
	err := longError()
	if err.RootCause().Error() != "test error" {
		t.Fatal("Root cause didn't match the root cause.", err.RootCause())
	}
}

func TestHumaneError_Facade(t *testing.T) {
	// Not much we can test here
	err := longError()
	lErr := LogError{
		HumaneError: *err,
	}
	jErr := JSONError{
		HumaneError: *err,
	}
	t.Log(lErr.Error())
	t.Log(jErr.Error())
}

func TestHumaneError_Coverall(t *testing.T) {
	err := CoverallError(errors.New("Some error"))
	t.Log(err.Error())
}

func longError() *HumaneError {
	err := errors.New("test error")
	he := Wrap(err, http.StatusNoContent, "user message", struct {
		Test string
	}{"testData"})
	return Wrap(he, http.StatusInternalServerError, "top message", struct {
		User string
		ID   string
	}{"Phil", "123"})
}
