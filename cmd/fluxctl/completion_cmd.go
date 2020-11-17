package main

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var (
	completionShells = map[string]func(out io.Writer, cmd *cobra.Command) error{
		"bash": runCompletionBash,
		"zsh":  runCompletionZsh,
		"fish": runCompletionFish,
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
		Short:                 "Output shell completion code for the specified shell (bash, zsh, or fish)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return newUsageError("please specify a shell")
			}

			if len(args) > 1 {
				sort.Strings(shells)
				return newUsageError(fmt.Sprintf("please specify one of the following shells: %s", strings.Join(shells, " ")))
			}

			run, found := completionShells[args[0]]
			if !found {
				return newUsageError(fmt.Sprintf("unsupported shell type %q", args[0]))
			}

			return run(cmd.OutOrStdout(), cmd)
		},
		ValidArgs: shells,
	}
}

func runCompletionBash(out io.Writer, fluxctl *cobra.Command) error {
	return fluxctl.Root().GenBashCompletion(out)
}

func runCompletionZsh(out io.Writer, fluxctl *cobra.Command) error {
	return fluxctl.Root().GenZshCompletion(out)
}

func runCompletionFish(out io.Writer, fluxctl *cobra.Command) error {
	return fluxctl.Root().GenFishCompletion(out, true)
}
