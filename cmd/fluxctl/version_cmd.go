package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version string

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Output the version of fluxctl",
		RunE: func(_ *cobra.Command, args []string) error {
			if version == "" {
				version = "unversioned"
			}
			fmt.Println(version)
			return nil
		},
	}
}
