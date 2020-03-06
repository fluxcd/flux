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

func checkExactlyOne(optsDescription string, supplied ...bool) error {
	found := false
	for _, s := range supplied {
		if found && s {
			return newUsageError("please supply only one of " + optsDescription)
		}
		found = found || s
	}

	if !found {
		return newUsageError("please supply exactly one of " + optsDescription)
	}

	return nil
}

var errorWantedNoArgs = newUsageError("expected no (non-flag) arguments")
var errorInvalidOutputFormat = newUsageError("invalid output format specified")
