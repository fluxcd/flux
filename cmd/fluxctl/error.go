package main

import (
	"errors"
	"fmt"
)

type usageError struct {
	error
}

func newUsageError(msg string) usageError {
	return usageError{error: errors.New(msg)}
}

func checkExactly(optsDescription string, desired int, supplied ...bool) error {
	if countTrue(supplied) != desired {
		return newUsageError(fmt.Sprintf("please supply exactly %d of %s", desired, optsDescription))
	}
	return nil
}

func checkAtMost(optsDescription string, atMost int, supplied ...bool) error {
	if countTrue(supplied) > atMost {
		return newUsageError(fmt.Sprintf("please supply at most %d of %s", atMost, optsDescription))
	}
	return nil
}

func checkAtLeast(optsDescription string, atLeast int, supplied ...bool) error {
	if countTrue(supplied) < atLeast {
		return newUsageError(fmt.Sprintf("please supply at most %d of %s", atLeast, optsDescription))
	}
	return nil
}

func countTrue(supplied []bool) int {
	truecount := 0
	for _, s := range supplied {
		if s {
			truecount += 1
		}
	}
	return truecount
}

var errorWantedNoArgs = newUsageError("expected no (non-flag) arguments")
var errorInvalidOutputFormat = newUsageError("invalid output format specified")
