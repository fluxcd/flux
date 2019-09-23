package main

import (
	"os"

	"github.com/pkg/errors"

	fluxerr "github.com/fluxcd/flux/pkg/errors"
)

func run(args []string) int {
	rootCmd := newRoot().Command()
	rootCmd.SetArgs(args)
	if cmd, err := rootCmd.ExecuteC(); err != nil {
		// Format flux-specific errors. They can come wrapped,
		// so we use the cause instead.
		if cause, ok := errors.Cause(err).(*fluxerr.Error); ok {
			cmd.Println("== Error ==\n\n" + cause.Help)
		} else {
			cmd.Println("Error: " + err.Error())
			cmd.Printf("Run '%v --help' for usage.\n", cmd.CommandPath())
		}
		return 1
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:]))
}
