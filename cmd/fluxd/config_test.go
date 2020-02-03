package main

import (
	"testing"

	"github.com/spf13/pflag"
)

func TestDefineEverything(t *testing.T) {
	flags := pflag.NewFlagSet("testflags", pflag.ContinueOnError)
	defineConfigFlags(flags, func(err error) {
		t.Error(err)
	})
}
