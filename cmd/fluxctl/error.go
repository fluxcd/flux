package main

import (
	"fmt"
)

type usageError struct {
	error
}

func (err *usageError) Error() string {
	return err.error.Error()
}

func newUsageError(err error) *usageError {
	return &usageError{error: err}
}

var (
	errorWantedNoArgs = newUsageError(fmt.Errorf("expected no (non-flag) arguments"))
)
