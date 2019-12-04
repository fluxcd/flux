package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type buildOpts struct {
}

func newBuild() *buildOpts {
	return &buildOpts{}
}

func (opts *buildOpts) Command() *cobra.Command {
	return &cobra.Command{
		Use:   "build",
		Short: "Generate manifests from a local directory as fluxd would",
		RunE:  build,
	}
}

func build(cmd *cobra.Command, args []string) error {
	fmt.Fprintf(os.Stderr, "building\n")
	return nil
}
