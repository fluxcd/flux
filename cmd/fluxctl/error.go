package main

import (
	"errors"
	"fmt"
	"strings"
)

type usageError struct {
	error
}

func newUsageError(msg string) usageError {
	return usageError{error: errors.New(msg)}
}

func mutuallyExclusive(opt1, opt2 string) error {
	return newUsageError(fmt.Sprintf("please supply only one of %s or %s", opt1, opt2))
}

func exactlyOne(opts ...string) error {
	var allButLast string
	if len(opts) > 2 {
		allButLast = strings.Join(opts[:len(opts)-1], ", ") + ","
	} else {
		allButLast = opts[0]
	}
	return newUsageError("please supply exactly one of " + allButLast + " or " + opts[len(opts)-1])
}

var (
	errorWantedNoArgs = newUsageError("expected no (non-flag) arguments")
)
