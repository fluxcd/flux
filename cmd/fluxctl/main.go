package main

import (
	"os"
	"strings"

	"github.com/pkg/errors"

	transport "github.com/weaveworks/flux/http"
)

func main() {
	rootCmd := newRoot().Command()
	if cmd, err := rootCmd.ExecuteC(); err != nil {
		err = errors.Cause(err)
		switch err := err.(type) {
		case usageError:
			cmd.Println("")
			cmd.Println(cmd.UsageString())
		case *transport.APIError:
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
		os.Exit(1)
	}
}
