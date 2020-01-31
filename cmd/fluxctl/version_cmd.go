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
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return errorWantedNoArgs
			}
			if version == "" {
				version = "unversioned"
			}
			fmt.Fprintln(cmd.OutOrStdout(), version)
			return nil
		},
	}
}
