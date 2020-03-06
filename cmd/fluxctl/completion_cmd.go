package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

var (
	completionShells = map[string]func(out io.Writer, cmd *cobra.Command) error{
		"bash": runCompletionBash,
		"zsh":  runCompletionZsh,
	}
)

func newCompletionCommand() *cobra.Command {
	shells := []string{}
	for s := range completionShells {
		shells = append(shells, s)
	}

	return &cobra.Command{
		Use:                   "completion SHELL",
		DisableFlagsInUseLine: true,
		Short:                 "Output shell completion code for the specified shell (bash or zsh)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return newUsageError("please specify a shell")
			}

			if len(args) > 1 {
				return newUsageError(fmt.Sprintf("please specify one of the following shells: %s", strings.Join(shells, " ")))
			}

			run, found := completionShells[args[0]]
			if !found {
				return newUsageError(fmt.Sprintf("unsupported shell type %q.", args[0]))
			}

			return run(cmd.OutOrStdout(), cmd.Parent())
		},
		ValidArgs: shells,
	}
}

func runCompletionBash(out io.Writer, fluxctl *cobra.Command) error {
	return fluxctl.GenBashCompletion(out)
}

func runCompletionZsh(out io.Writer, fluxctl *cobra.Command) error {
	return fluxctl.GenZshCompletion(out)
}
