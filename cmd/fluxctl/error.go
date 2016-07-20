package main

import (
	"errors"
)

type usageError struct {
	error
}

func newUsageError(msg string) usageError {
	return usageError{error: errors.New(msg)}
}

var (
	errorWantedNoArgs = newUsageError("expected no (non-flag) arguments")
)
