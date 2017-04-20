package main

import (
	"os"

	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
)

func run(args []string) int {
	rootCmd := newRoot().Command()
	rootCmd.SetArgs(args)
	if cmd, err := rootCmd.ExecuteC(); err != nil {
		err = errors.Cause(err)
		switch err := err.(type) {
		case *flux.BaseError:
			cmd.Println("== Error ==\n\n" + err.Help)
		default:
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
