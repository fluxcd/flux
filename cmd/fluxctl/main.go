package main

import (
	"io"
	"os"
	"strings"

	"github.com/pkg/errors"

	"github.com/weaveworks/flux/http/error"
)

func run(args []string, stderr io.Writer) int {
	rootCmd := newRoot().Command()
	rootCmd.SetArgs(args)
	rootCmd.SetOutput(stderr)
	if cmd, err := rootCmd.ExecuteC(); err != nil {
		err = errors.Cause(err)
		switch err := err.(type) {
		case *httperror.APIError:
			switch {
			case err.IsMissing():
				cmd.Println(strings.Join([]string{
					"Error: Service API endpoint not found. This usually means that there is a mismatch between the client and the service. Please visit",
					"    https://github.com/weaveworks/flux/releases",
					"to download a new release of the client."}, "\n"))
			default:
				cmd.Println("Problem communicating with the service: ", err.Error())
				cmd.Printf("Run '%v --help' for usage.\n", cmd.CommandPath())
			}
		default:
			cmd.Println("Error: ", err.Error())
			cmd.Printf("Run '%v --help' for usage.\n", cmd.CommandPath())
		}
		return 1
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}
